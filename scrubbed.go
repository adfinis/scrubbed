package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

type (
	Config struct {
		LogLevel       slog.Level `env:"SCRUBBED_LOG_LEVEL" envDefault:"INFO"`
		RedactedString string     `env:"SCRUBBED_REDACTED_STRING" envDefault:"REDACTED"`

		AlertLabels       []string `env:"SCRUBBED_ALERT_LABELS" envDefault:"alertname severity" envSeparator:" "`
		AlertAnnotations  []string `env:"SCRUBBED_ALERT_ANNOTATIONS" envDefault:"" envSeparator:" "`
		GroupLabels       []string `env:"SCRUBBED_GROUP_LABELS" envDefault:"" envSeparator:" "`
		CommonLabels      []string `env:"SCRUBBED_COMMON_LABELS" envDefault:"alertname severity" envSeparator:" "`
		CommonAnnotations []string `env:"SCRUBBED_COMMON_ANNOTATIONS" envDefault:"" envSeparator:" "`

		Host        string `env:"SCRUBBED_LISTEN_HOST" envDefault:"127.0.0.1"`
		Port        string `env:"SCRUBBED_LISTEN_PORT" envDefault:"8080"`
		TLSEnable   bool   `env:"SCRUBBED_LISTEN_TLS_ENABLE" envDefault:"FALSE"`
		TLSCertPath string `env:"SCRUBBED_LISTEN_TLS_CERT_PATH" envDefault:"tls.crt"`
		TLSKeyPath  string `env:"SCRUBBED_LISTEN_TLS_KEY_PATH" envDefault:"tls.key"`
		Url         string `env:"SCRUBBED_DESTINATION_URL,required"`
	}

	statusResponse struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	// HookMessage is the message we receive from Alertmanager.
	HookMessage struct {
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
	Alert struct {
		Status       string            `json:"status"             validate:"required"`
		Labels       map[string]string `json:"labels"             validate:"required"`
		Annotations  map[string]string `json:"annotations"        validate:"required"`
		StartsAt     string            `json:"startsAt,omitempty" validate:"required"`
		EndsAt       string            `json:"endsAt,omitempty"   validate:"required"`
		GeneratorURL string            `json:"generatorURL"       validate:"required"`
		Fingerprint  string            `json:"fingerprint"        validate:"required"`
	}
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Parsing environment variables", "error", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	router := mux.NewRouter()
	router.HandleFunc("/webhook", webhookHandler(cfg)).Methods("POST")
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

func webhookHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var alert HookMessage

		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if mediaType != "application/json" {
			msg := "Content-Type header is not application/json"
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusUnsupportedMediaType)

			return
		}

		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			msg := fmt.Sprintf("Failed to decode JSON: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusBadRequest)

			return
		}

		slog.Debug("Received JSON: " + toJSONString(alert))

		validate := validator.New()
		err := validate.Struct(alert)

		if err != nil {
			msg := fmt.Sprintf("Failed to validate JSON structure: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusBadRequest)

			return
		}

		scrub(&alert, cfg)

		slog.Debug("Sending JSON: " + toJSONString(alert))

		client := &http.Client{Timeout: 60 * time.Second}
		req, err := http.NewRequest("POST", cfg.Url, strings.NewReader(toJSONString(alert)))

		if err != nil {
			msg := fmt.Sprintf("Failed to create request: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)

			return
		}

		req.Header = r.Header
		resp, err := client.Do(req)

		if err != nil {
			msg := fmt.Sprintf("Failed to send request: %v", err)
			slog.Error(msg)
			http.Error(w, toJSONString(statusResponse{Status: "error", Message: msg}), http.StatusInternalServerError)

			return
		}

		defer resp.Body.Close()

		msg := fmt.Sprintf("alert received and processed with code %d", resp.StatusCode)

		response := statusResponse{
			Status:  "success",
			Message: msg,
		}

		slog.Info(msg)
		w.WriteHeader(resp.StatusCode)

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
