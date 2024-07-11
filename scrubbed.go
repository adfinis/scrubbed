package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env"
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
	TruncatedAlerts   int               `json:"truncatedAlerts" validate:"number"`
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
	StartsAt     string            `json:"startsAt,omitempty" validate:"required"`
	EndsAt       string            `json:"endsAt,omitempty"   validate:"required"`
	GeneratorURL string            `json:"generatorURL"       validate:"required"`
	Fingerprint  string            `json:"fingerprint"        validate:"required"`
}

func main() {

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Parsing environment variables", "error", err)
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

	slog.Info("Starting server", "Host", cfg.Host, "Port", cfg.Port)
	{
		var err error

		server := &http.Server{
			Addr:         cfg.Host + ":" + cfg.Port,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			Handler:      router,
		}

		if cfg.TLSEnable {
			err = server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil {
			slog.Error("Server failed", "error", err)
		}
	}
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
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusUnsupportedMediaType)

			return
		}

		// Decode body to HookMessage struct.
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			msg := fmt.Sprintf("Failed to decode JSON: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusBadRequest)

			return
		}

		slog.Debug("Received JSON: " + toJSONString(alert))

		// Validate HookMessage.
		validate := validator.New()
		err := validate.Struct(alert)

		if err != nil {
			msg := fmt.Sprintf("Failed to validate JSON structure: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusBadRequest)

			return
		}

		// Scrub it.
		scrub(&alert, cfg)

		slog.Debug("Sending JSON: " + toJSONString(alert))

		// Post it to upstream URL.
		resp, err := postFunc(cfg.Url, r.Header, strings.NewReader(toJSONString(alert)))
		if err != nil {
			msg := fmt.Sprintf("Failed to post to webhook: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("Couldn't read received body %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

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
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)

			return
		}
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("OK")); err != nil {
		msg := fmt.Sprintf("Failed to write a response: %v", err)
		slog.Error(msg)
		http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)

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

func toJSONString(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	return string(bytes)
}
