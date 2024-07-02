#!/bin/env python3

from flask import Flask, request, jsonify
import requests
import logging
import os

app = Flask(__name__)

# Map the string level to a logging level
log_level_map = {
    "DEBUG": logging.DEBUG,
    "INFO": logging.INFO,
    "WARNING": logging.WARNING,
    "ERROR": logging.ERROR,
    "CRITICAL": logging.CRITICAL,
}

log_level = os.getenv("SCRUBBED_LOG_LEVEL", "INFO").upper()
logging.basicConfig(level=log_level_map.get(log_level, logging.WARNING))

logger = logging.getLogger("scrubbed")
logger.info("hello")

# Replace non whitelisted values with REDACTED_STRING
REDACTED_STRING = os.environ.get("SCRUBBED_REDACTED_STRING", "REDACTED")

# Whitelist filtering configuration
ALERT_LABELS = os.environ.get("SCRUBBED_ALERT_LABELS", "alertname severity").split()
ALERT_ANNOTATIONS = os.environ.get("SCRUBBED_ALERT_ANNOTATIONS", "").split()
GROUP_LABELS = os.environ.get("SCRUBBED_GROUP_LABELS", "").split()
COMMON_LABELS = os.environ.get("SCRUBBED_COMMON_LABELS", "alertname severity").split()
COMMON_ANNOTATIONS = os.environ.get("SCRUBBED_COMMON_ANNOTATIONS", "").split()

# Service configuration
PORT = os.environ.get("SCRUBBED_LISTEN_PORT", 8080)
URL = os.environ.get("SCRUBBED_DESTINATION_URL", "http://localhost:6725")


def redact_fields(fields, keys_to_keep):
    return {
        key: (fields[key] if key in keys_to_keep else REDACTED_STRING) for key in fields
    }


def scrub(alert):
    for a in alert["alerts"]:
        a["labels"] = redact_fields(a["labels"], ALERT_LABELS)
        a["annotations"] = redact_fields(a["annotations"], ALERT_ANNOTATIONS)
        a["generatorURL"] = REDACTED_STRING
    alert["groupLabels"] = redact_fields(alert["groupLabels"], GROUP_LABELS)
    alert["commonLabels"] = redact_fields(alert["commonLabels"], COMMON_LABELS)
    alert["commonAnnotations"] = redact_fields(
        alert["commonAnnotations"], COMMON_ANNOTATIONS
    )
    alert["externalURL"] = REDACTED_STRING
    alert["groupKey"] = REDACTED_STRING


@app.post("/webhook")
def webhook():
    if request.is_json:
        try:
            alert = request.get_json()

            scrub(alert)

            logger.debug(f"sending:\n{alert}")

            session = requests.Session()

            # Copy headers
            session.headers.clear()
            for h in request.headers.keys():
                session.headers[h] = request.headers.get(h)

            r = session.post(URL, json=alert)
            msg = "alert received and processed"
            response = {
                "status": "success",
                "message": f"{msg}, status code {r.status_code}",
            }
            logger.info(f"{msg} with code {r.status_code}")
            return jsonify(response), r.status_code
        except Exception as e:
            response = {
                "status": "error",
                "message": str(e),
            }
            logger.error(str(e))
            return jsonify(response), 500
    else:
        msg = "request must be in JSON format"
        response = {
            "status": "error",
            "message": msg,
        }
        logger.error(msg)
        return jsonify(response), 400


if __name__ == "__main__":
    from waitress import serve

    serve(app, host="0.0.0.0", port=PORT)
