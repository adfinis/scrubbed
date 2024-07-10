package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

var (
	cfg = Config{
		RedactedString: "REDACTED",
		AlertLabels:    []string{"alertname", "severity"},
		CommonLabels:   []string{"alertname", "severity"},
	}

	validAlertLabels = map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    "foo",
		"namespace":  "monitoring",
		"node":       "node.foo.example.com",
		"prometheus": "monitoring/k8s",
		"severity":   "critical",
	}

	validAlertLabelsRedacted = map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    cfg.RedactedString,
		"namespace":  cfg.RedactedString,
		"node":       cfg.RedactedString,
		"prometheus": cfg.RedactedString,
		"severity":   "critical",
	}

	validAlertAnnotations = map[string]string{
		"description": "Instance https://server.example.org has been down for over 5m. Job: http_checks",
		"summary":     "BlackBox Probe Failure: https://server.example.org",
	}

	validAlertAnnotationsRedacted = map[string]string{
		"description": cfg.RedactedString,
		"summary":     cfg.RedactedString,
	}

	validAlert = Alert{
		Status:       "firing",
		Labels:       validAlertLabels,
		Annotations:  validAlertAnnotations,
		StartsAt:     "2023-02-06T13:08:45.828Z",
		EndsAt:       "0001-01-01T00:00:00Z",
		GeneratorURL: "https://console.apps.example.com/monitoring",
		Fingerprint:  "1a30ba71cca2921f",
	}

	validAlertRedacted = Alert{
		Status:       "firing",
		Labels:       validAlertLabelsRedacted,
		Annotations:  validAlertAnnotationsRedacted,
		StartsAt:     "2023-02-06T13:08:45.828Z",
		EndsAt:       "0001-01-01T00:00:00Z",
		GeneratorURL: cfg.RedactedString,
		Fingerprint:  "1a30ba71cca2921f",
	}

	validGroupLabels = map[string]string{
		"namespace": "monitoring",
	}

	validGroupLabelsRedacted = map[string]string{
		"namespace": cfg.RedactedString,
	}

	validCommonLabels = map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    "foo",
		"namespace":  "monitoring",
		"prometheus": "monitoring/k8s",
		"severity":   "critical",
	}

	validCommonLabelsRedacted = map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    cfg.RedactedString,
		"namespace":  cfg.RedactedString,
		"prometheus": cfg.RedactedString,
		"severity":   "critical",
	}

	validHookMessage = HookMessage{
		Receiver:        "default",
		Status:          "firing",
		Alerts:          []Alert{validAlert},
		GroupLabels:     validGroupLabels,
		CommonLabels:    validCommonLabels,
		ExternalURL:     "https://console.apps.example.com/monitoring",
		Version:         "4",
		GroupKey:        "{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}",
		TruncatedAlerts: 0,
	}

	validHookMessageRedacted = HookMessage{
		Receiver:        "default",
		Status:          "firing",
		Alerts:          []Alert{validAlertRedacted},
		GroupLabels:     validGroupLabelsRedacted,
		CommonLabels:    validCommonLabelsRedacted,
		ExternalURL:     cfg.RedactedString,
		Version:         "4",
		GroupKey:        cfg.RedactedString,
		TruncatedAlerts: 0,
	}

	validHookMessageString = `{"version":"4","groupKey":"{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}","truncatedAlerts":0,"status":"firing","receiver":"default","groupLabels":{"namespace":"monitoring"},"commonLabels":{"alertname":"ProbeFailure","cluster":"foo","namespace":"monitoring","prometheus":"monitoring/k8s","severity":"critical"},"commonAnnotations":null,"externalURL":"https://console.apps.example.com/monitoring","alerts":[{"status":"firing","labels":{"alertname":"ProbeFailure","cluster":"foo","namespace":"monitoring","node":"node.foo.example.com","prometheus":"monitoring/k8s","severity":"critical"},"annotations":{"description":"Instance https://server.example.org has been down for over 5m. Job: http_checks","summary":"BlackBox Probe Failure: https://server.example.org"},"startsAt":"2023-02-06T13:08:45.828Z","EndsAt":"0001-01-01T00:00:00Z","generatorURL":"https://console.apps.example.com/monitoring","fingerprint":"1a30ba71cca2921f"}]}`
)

func TestToJSONStringValid(t *testing.T) {
	t.Parallel()

	resultString := toJSONString(validHookMessage)

	if resultString != validHookMessageString {
		t.Fatalf(`toJSONString(%v) = %s, want "%s"`, validHookMessage, resultString, validHookMessageString)
	}
}

func TestToJSONStringNil(t *testing.T) {
	t.Parallel()

	resultString := toJSONString(nil)
	testString := "null"

	if resultString != testString {
		t.Fatalf(`toJSONString(%v) = %s, want %s`, nil, resultString, testString)
	}
}

func TestRedactFields(t *testing.T) {
	t.Parallel()

	resultMap := validAlertLabels
	redactFields(&resultMap, cfg.AlertLabels, cfg.RedactedString)

	if !reflect.DeepEqual(resultMap, validAlertLabelsRedacted) {
		t.Fatalf(`redactFields(%v) = %v, want %v`, validAlertLabels, resultMap, validAlertLabelsRedacted)
	}
}

func TestScrub(t *testing.T) {
	t.Parallel()

	resultHookMessage := validHookMessage
	scrub(&resultHookMessage, cfg)

	if !reflect.DeepEqual(resultHookMessage, validHookMessageRedacted) {
		t.Fatalf(`scrub(%v) = %v, want %v`, validHookMessage, resultHookMessage, validHookMessageRedacted)
	}
}

func TestHealthCheckHandler(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	healthCheckHandler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf(`healthCheckHandler return code is %v, expected %v`, w.Code, http.StatusOK)
	}
}
