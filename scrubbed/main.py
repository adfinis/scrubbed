#!/bin/env python3
"""Receive alerts, scrub them from private data, and forward them."""

from __future__ import annotations

import logging
import os

import requests
from flask import Flask, jsonify, request

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
HOST = os.environ.get("SCRUBBED_LISTEN_HOST", "127.0.0.1")
PORT = os.environ.get("SCRUBBED_LISTEN_PORT", 8080)
URL = os.environ.get("SCRUBBED_DESTINATION_URL", "http://localhost:6725")
TIMEOUT = 60


def redact_fields(fields: dict[str, str], keys_to_keep: list[str]):
    """Scrub individual keys in an alert."""
    return {
        key: (fields[key] if key in keys_to_keep else REDACTED_STRING) for key in fields
    }


def scrub(alert: dict):
    """Scrub several alerts."""
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
    """Receive an alert message, scrub it and forward it to a alert receiver."""
    if request.is_json:
        try:
            alert = request.get_json()

            scrub(alert)

            logger.debug("sending: \n%s", alert)

            r = requests.post(
                URL,
                json=alert,
                headers=request.headers,
                timeout=TIMEOUT,
            )
            msg = "alert received and processed"
            response = {
                "status": "success",
                "message": f"{msg}, status code {r.status_code}",
            }
            logger.info("%s with code %s", msg, r.status_code)
            return jsonify(response), r.status_code
        except Exception as e:
            response = {
                "status": "error",
                "message": str(e),
            }
            logger.exception(msg=str(e))
            return jsonify(response), 500
    else:
        msg = "request must be in JSON format"
        response = {
            "status": "error",
            "message": msg,
        }
        logger.error(msg)
        return jsonify(response), 400


@app.route("/healthz")
def health_check():  # pragma: nocover
    """Endpoint for health probes."""
    return "OK", 200


if __name__ == "__main__":  # pragma: nocover
    from waitress import serve

    serve(app, host=HOST, port=PORT)
