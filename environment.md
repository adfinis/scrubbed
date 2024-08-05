# Environment Variables

## Config

 - `SCRUBBED_LOG_LEVEL` (default: `INFO`) - Set logging level
 - `SCRUBBED_REDACTED_STRING` (default: `REDACTED`) - Set literal string to redact values with
 - `SCRUBBED_ALERT_LABELS` (separated by ` `, default: `alertname severity`) - Space separated alert labels to keep
 - `SCRUBBED_ALERT_ANNOTATIONS` (separated by ` `) - Space separated alert annotations to keep
 - `SCRUBBED_GROUP_LABELS` (separated by ` `) - Space separated group labels to keep
 - `SCRUBBED_COMMON_LABELS` (separated by ` `, default: `alertname severity`) - Space separated common labels to keep
 - `SCRUBBED_COMMON_ANNOTATIONS` (separated by ` `) - Space separated common annotations to keep
 - `SCRUBBED_LISTEN_HOST` (default: `127.0.0.1`) - Service listener address
 - `SCRUBBED_LISTEN_PORT` (default: `8080`) - Service listener port
 - `SCRUBBED_LISTEN_TLS_ENABLE` (default: `FALSE`) - Enable TLS
 - `SCRUBBED_LISTEN_TLS_CERT_PATH` (default: `tls.crt`) - Path to TLS certificate
 - `SCRUBBED_LISTEN_TLS_KEY_PATH` (default: `tls.key`) - Path to TLS key
 - `SCRUBBED_DESTINATION_URL` (**required**) - Webhook destination URL e.g. https://monitoring.example.com/webhook?foo=bar

