import json

import pytest
from werkzeug.test import Client

from scrubbed.main import app


@pytest.fixture()
def client() -> Client:
    """Test client for calling server endpoints."""
    return Client(app)


def test_invalid_get(client):
    response = client.get("/webhook")

    assert response.status == "405 METHOD NOT ALLOWED"
    assert response.content_type == "text/html; charset=utf-8"


def test_invalid_content_type(client):
    response = client.post(
        "/webhook", content_type="text/html", data="<h1>Hello World!</h1>"
    )

    assert response.status == "400 BAD REQUEST"
    assert response.content_type == "application/json"
    assert response.json.get("message") == "request must be in JSON format"
    assert response.json.get("status") == "error"


def test_invalid_data(client):
    response = client.post(
        "/webhook", content_type="application/json", data="<h1>Hello World!</h1>"
    )

    assert response.status == "500 INTERNAL SERVER ERROR"
    assert response.content_type == "application/json"
    assert (
        response.json.get("message")
        == "400 Bad Request: The browser (or proxy) sent a request that this server could not understand."  # noqa: E501
    )
    assert response.json.get("status") == "error"


def test_scrubbing_alert(requests_mock, client):
    upstream_request = requests_mock.post("http://localhost:6725")

    alerts = {
        "version": "4",
        "groupKey": "groupkey",
        "truncatedAlerts": 0,
        "status": "firing",
        "receiver": "test",
        "groupLabels": {
            "KEY": "SECRET",
        },
        "commonLabels": {
            "KEY": "SECRET",
        },
        "commonAnnotations": {
            "KEY": "SECRET",
        },
        "externalURL": "https://SECRET.alertmanager.example.com",
        "alerts": [
            {
                "status": "firing",
                "labels": {"KEY": "SECRET"},
                "annotations": {"KEY": "SECRET"},
                "startsAt": "<rfc3339>",
                "endsAt": "<rfc3339>",
                "generatorURL": "https://SECRET.generator.example.com",
                "fingerprint": "fingerprint",
            }
        ],
    }
    response = client.post(
        "/webhook",
        content_type="application/json",
        headers={
            "KEY": "SECRET",
        },
        data=json.dumps(alerts),
    )

    assert response.status == "200 OK"
    assert response.content_type == "application/json"
    assert upstream_request.call_count == 1
    assert "SECRET" not in upstream_request.last_request.text
    assert upstream_request.last_request.json() == {
        "version": "4",
        "groupKey": "REDACTED",
        "truncatedAlerts": 0,
        "status": "firing",
        "receiver": "test",
        "groupLabels": {
            "KEY": "REDACTED",
        },
        "commonLabels": {
            "KEY": "REDACTED",
        },
        "commonAnnotations": {
            "KEY": "REDACTED",
        },
        "externalURL": "REDACTED",
        "alerts": [
            {
                "annotations": {
                    "KEY": "REDACTED",
                },
                "endsAt": "<rfc3339>",
                "fingerprint": "fingerprint",
                "generatorURL": "REDACTED",
                "labels": {
                    "KEY": "REDACTED",
                },
                "startsAt": "<rfc3339>",
                "status": "firing",
            },
        ],
    }
    assert upstream_request.last_request.headers == {
        "Host": "localhost",
        "Content-Type": "application/json",
        "Content-Length": "451",
        "Key": "SECRET",  # TODO: redact this?
    }
