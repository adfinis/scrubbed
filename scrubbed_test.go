package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Poor man's ternary operator.
func If[T any](cond bool, vtrue, vfalse T) T {
	if cond {
		return vtrue
	}
	return vfalse
}

func createValidAlertLabels(redacted bool) map[string]string {
	return map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    If(redacted, cfg.RedactedString, "foo"),
		"namespace":  If(redacted, cfg.RedactedString, "monitoring"),
		"node":       If(redacted, cfg.RedactedString, "node.foo.example.com"),
		"prometheus": If(redacted, cfg.RedactedString, "monitoring/k8s"),
		"severity":   "critical",
	}
}

func createValidAlertAnnotations(redacted bool) map[string]string {
	return map[string]string{
		"description": If(redacted, cfg.RedactedString, "Instance https://server.example.org has been down for over 5m. Job: http_checks"),
		"summary":     If(redacted, cfg.RedactedString, "BlackBox Probe Failure: https://server.example.org"),
	}
}

func createValidAlert(redacted bool) Alert {
	return Alert{
		Status:       "firing",
		Labels:       createValidAlertLabels(redacted),
		Annotations:  createValidAlertAnnotations(redacted),
		StartsAt:     "2023-02-06T13:08:45.828Z",
		EndsAt:       "0001-01-01T00:00:00Z",
		GeneratorURL: If(redacted, cfg.RedactedString, "https://console.apps.example.com/monitoring"),
		Fingerprint:  "1a30ba71cca2921f",
	}
}

func createValidGroupLabels(redacted bool) map[string]string {
	return map[string]string{
		"namespace": If(redacted, cfg.RedactedString, "monitoring"),
	}
}

func createValidCommonLabels(redacted bool) map[string]string {
	return map[string]string{
		"alertname":  "ProbeFailure",
		"cluster":    If(redacted, cfg.RedactedString, "foo"),
		"namespace":  If(redacted, cfg.RedactedString, "monitoring"),
		"prometheus": If(redacted, cfg.RedactedString, "monitoring/k8s"),
		"severity":   "critical",
	}
}

func createValidHookMessage(redacted bool) HookMessage {
	return HookMessage{
		Receiver:        "default",
		Status:          "firing",
		Alerts:          []Alert{createValidAlert(redacted)},
		GroupLabels:     createValidGroupLabels(redacted),
		CommonLabels:    createValidCommonLabels(redacted),
		ExternalURL:     If(redacted, cfg.RedactedString, "https://console.apps.example.com/monitoring"),
		Version:         "4",
		GroupKey:        If(redacted, cfg.RedactedString, "{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}"),
		TruncatedAlerts: 0,
	}
}

var (
	cfg = Config{
		RedactedString: "REDACTED",
		AlertLabels:    []string{"alertname", "severity"},
		CommonLabels:   []string{"alertname", "severity"},
		Url:            "http://foo.bar.baz:444/hello",
	}

	validHookMessageString         = `{"version":"4","groupKey":"{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}","truncatedAlerts":0,"status":"firing","receiver":"default","groupLabels":{"namespace":"monitoring"},"commonLabels":{"alertname":"ProbeFailure","cluster":"foo","namespace":"monitoring","prometheus":"monitoring/k8s","severity":"critical"},"commonAnnotations":null,"externalURL":"https://console.apps.example.com/monitoring","alerts":[{"status":"firing","labels":{"alertname":"ProbeFailure","cluster":"foo","namespace":"monitoring","node":"node.foo.example.com","prometheus":"monitoring/k8s","severity":"critical"},"annotations":{"description":"Instance https://server.example.org has been down for over 5m. Job: http_checks","summary":"BlackBox Probe Failure: https://server.example.org"},"startsAt":"2023-02-06T13:08:45.828Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"https://console.apps.example.com/monitoring","fingerprint":"1a30ba71cca2921f"}]}`
	validHookMessageStringRedacted = `{"version":"4","groupKey":"REDACTED","truncatedAlerts":0,"status":"firing","receiver":"default","groupLabels":{"namespace":"REDACTED"},"commonLabels":{"alertname":"ProbeFailure","cluster":"REDACTED","namespace":"REDACTED","prometheus":"REDACTED","severity":"critical"},"commonAnnotations":null,"externalURL":"REDACTED","alerts":[{"status":"firing","labels":{"alertname":"ProbeFailure","cluster":"REDACTED","namespace":"REDACTED","node":"REDACTED","prometheus":"REDACTED","severity":"critical"},"annotations":{"description":"REDACTED","summary":"REDACTED"},"startsAt":"2023-02-06T13:08:45.828Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"REDACTED","fingerprint":"1a30ba71cca2921f"}]}`
)

func TestToJSONStringValid(t *testing.T) {
	t.Parallel()

	resultString := toJSONString(createValidHookMessage(false))

	if resultString != validHookMessageString {
		t.Fatalf(`toJSONString(%v) = %s, want "%s"`, createValidHookMessage(false), resultString, validHookMessageString)
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

	resultMap := createValidAlertLabels(false)
	expectMap := createValidAlertLabels(true)

	redactFields(&resultMap, cfg.AlertLabels, cfg.RedactedString)

	if !reflect.DeepEqual(resultMap, expectMap) {
		t.Fatalf(`redactFields(%v) = %v, want %v`, createValidAlertLabels(false), resultMap, expectMap)
	}
}

func TestScrub(t *testing.T) {
	t.Parallel()

	resultHookMessage := createValidHookMessage(false)
	expectHookMessage := createValidHookMessage(true)

	scrub(&resultHookMessage, cfg)

	if !reflect.DeepEqual(resultHookMessage, expectHookMessage) {
		t.Fatalf(`scrub(%v) = %v, want %v`, createValidHookMessage(false), resultHookMessage, expectHookMessage)
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

func postToWebhookMock(url string, header http.Header, body io.Reader) (*http.Response, error) {

	resp := http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("OK")),
	}

	return &resp, nil
}

func TestWebhookHandler(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("POST", "/webhook", strings.NewReader(validHookMessageString))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	webhookHandler(cfg, postToWebhookMock)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf(`webhookHandler return code is %v, expected %v`, w.Code, http.StatusOK)
	}
}

func TestWebhookHandlerInvalidContentType(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("POST", "/webhook", strings.NewReader(validHookMessageString))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	webhookHandler(cfg, postToWebhookMock)(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(`webhookHandler return code is %v, expected %v`, w.Code, http.StatusUnsupportedMediaType)
	}
}

func TestWebhookHandlerInvalidJSON(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("POST", "/webhook", strings.NewReader(`{"this": "json", "is": "invalid}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	webhookHandler(cfg, postToWebhookMock)(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf(`webhookHandler return code is %v, expected %v`, w.Code, http.StatusBadRequest)
	}
}
