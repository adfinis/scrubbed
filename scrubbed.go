package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

//go:generate envdoc --output environment.md
type Config struct {
	// Set logging level
	LogLevel string `env:"SCRUBBED_LOG_LEVEL" envDefault:"INFO"`
	// Set literal string to redact values with
	RedactedString string `env:"SCRUBBED_REDACTED_STRING" envDefault:"REDACTED"`
	// Space separated alert labels to keep
	AlertLabels []string `env:"SCRUBBED_ALERT_LABELS" envDefault:"alertname severity" envSeparator:" "`
	// Space separated alert annotations to keep
	AlertAnnotations []string `env:"SCRUBBED_ALERT_ANNOTATIONS" envDefault:"" envSeparator:" "`
	// Space separated group labels to keep
	GroupLabels []string `env:"SCRUBBED_GROUP_LABELS" envDefault:"" envSeparator:" "`
	// Space separated common labels to keep
	CommonLabels []string `env:"SCRUBBED_COMMON_LABELS" envDefault:"alertname severity" envSeparator:" "`
	// Space separated common annotations to keep
	CommonAnnotations []string `env:"SCRUBBED_COMMON_ANNOTATIONS" envDefault:"" envSeparator:" "`
	// Service listener address
	Host string `env:"SCRUBBED_LISTEN_HOST" envDefault:"127.0.0.1"`
	// Service listener port
	Port string `env:"SCRUBBED_LISTEN_PORT" envDefault:"8080"`
	// Enable TLS
	TLSEnable bool `env:"SCRUBBED_LISTEN_TLS_ENABLE" envDefault:"FALSE"`
	// Path to TLS certificate
	TLSCertPath string `env:"SCRUBBED_LISTEN_TLS_CERT_PATH" envDefault:"tls.crt"`
	// Path to TLS key
	TLSKeyPath string `env:"SCRUBBED_LISTEN_TLS_KEY_PATH" envDefault:"tls.key"`
	// Webhook destination URL e.g. https://monitoring.example.com/webhook?foo=bar
	Url string `env:"SCRUBBED_DESTINATION_URL,required"`
}

type statusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HookMessage is the message we receive from Alertmanager.
type HookMessage struct {
	Version           string            `json:"version" validate:"required"`
	GroupKey          string            `json:"groupKey" validate:"required"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status" validate:"required"`
	Receiver          string            `json:"receiver" validate:"required"`
	GroupLabels       map[string]string `json:"groupLabels" validate:"required"`
	CommonLabels      map[string]string `json:"commonLabels" validate:"required"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL" validate:"required"`
	Alerts            []Alert           `json:"alerts" validate:"required"`
}

// Alert is a single alert.
type Alert struct {
	Status       string            `json:"status"             validate:"required"`
	Labels       map[string]string `json:"labels"             validate:"required"`
	Annotations  map[string]string `json:"annotations"        validate:"required"`
	StartsAt     time.Time         `json:"startsAt,omitempty" validate:"required"`
	EndsAt       time.Time         `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL"       validate:"required"`
	Fingerprint  string            `json:"fingerprint"        validate:"required"`
}

func main() {

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}

	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Warn("Couldn't parse SCRUBBED_LOG_LEVEL, defaulting to INFO")
	}
	slog.Info("Logging setup", "level", level)

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	router := mux.NewRouter()
	router.HandleFunc("/webhook", webhookHandler(cfg, postToWebhook)).Methods("POST")
	router.HandleFunc("/healthz", healthCheckHandler).Methods("GET")

	server := &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      router,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Channel to listen for OS signals.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine.
	go func() {
		slog.Info("Starting server", "Host", cfg.Host, "Port", cfg.Port)

		var err error
		if cfg.TLSEnable {
			err = server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
		}
	}()

	// Block until we receive a signal.
	sig := <-quit
	slog.Info("Shutting down server", "signal", sig)

	// Create a context with a 5-second timeout for graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited gracefully")
}

func postToWebhook(url string, header http.Header, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header = header

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	return resp, err
}

func webhookHandler(cfg Config, postFunc func(string, http.Header, io.Reader) (*http.Response, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var alert HookMessage

		// Check for correct Content-type header.
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if mediaType != "application/json" {
			msg := "Content-Type header is not application/json"
			slog.Error(msg)
			writeError(w, msg, http.StatusUnsupportedMediaType)

			return
		}

		// Decode body to HookMessage struct with a 128KB limit.
		r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			msg := fmt.Sprintf("Failed to decode JSON: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusBadRequest)

			return
		}

		jsonStr, err := toJSONString(alert)
		if err != nil {
			msg := fmt.Sprintf("Failed to marshal alert to JSON: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusInternalServerError)
			return
		}
		slog.Debug("Received JSON: " + jsonStr)

		// Validate HookMessage.
		validate := validator.New()
		err = validate.Struct(alert)

		if err != nil {
			msg := fmt.Sprintf("Failed to validate JSON structure: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusBadRequest)

			return
		}

		// Scrub it.
		scrub(&alert, cfg)

		jsonStr, err = toJSONString(alert)
		if err != nil {
			msg := fmt.Sprintf("Failed to marshal scrubbed alert to JSON: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusInternalServerError)
			return
		}
		slog.Debug("Sending JSON: " + jsonStr)

		// Post it to upstream URL.
		resp, err := postFunc(cfg.Url, r.Header, strings.NewReader(jsonStr))
		if err != nil {
			msg := fmt.Sprintf("Failed to post to webhook: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("Couldn't read received body %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusInternalServerError)
			return
		}

		// Prepare response to original webhook caller based on response we receive.

		status := "success"
		if resp.StatusCode != 200 {
			status = "warning"
			slog.Warn("Received response from upstream", "code", resp.StatusCode, "body", body)
		} else {
			slog.Info("Received response from upstream", "code", resp.StatusCode, "body", body)
		}

		response := statusResponse{
			Status:  status,
			Message: fmt.Sprintf("Alert received and processed with code %d and body %s", resp.StatusCode, body),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			msg := fmt.Sprintf("Failed to encode: %v", err)
			slog.Error(msg)
			writeError(w, msg, http.StatusInternalServerError)

			return
		}
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("Failed to write health check response", "error", err)

		return
	}
}

func scrub(alert *HookMessage, cfg Config) {
	for idx := range alert.Alerts {
		redactFields(&alert.Alerts[idx].Labels, cfg.AlertLabels, cfg.RedactedString)
		redactFields(&alert.Alerts[idx].Annotations, cfg.AlertAnnotations, cfg.RedactedString)
		alert.Alerts[idx].GeneratorURL = cfg.RedactedString
	}

	redactFields(&alert.GroupLabels, cfg.GroupLabels, cfg.RedactedString)
	redactFields(&alert.CommonLabels, cfg.CommonLabels, cfg.RedactedString)
	redactFields(&alert.CommonAnnotations, cfg.CommonAnnotations, cfg.RedactedString)
	alert.ExternalURL = cfg.RedactedString
	alert.GroupKey = cfg.RedactedString
}

func redactFields(fields *map[string]string, keysToKeep []string, redactedString string) {
	for key := range *fields {
		if !contains(keysToKeep, key) {
			(*fields)[key] = redactedString
		}
	}
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}

	return false
}

func toJSONString(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(bytes), nil
}

func writeError(w http.ResponseWriter, msg string, statusCode int) {
	jsonStr, err := toJSONString(statusResponse{Status: "error", Message: msg})
	if err != nil {
		http.Error(w, `{"status":"error","message":"internal error"}`, http.StatusInternalServerError)
		return
	}
	http.Error(w, jsonStr, statusCode)
}
