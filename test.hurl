POST http://localhost:8080/webhook
User-Agent: Alertmanager/0.25.0
Accept:
{
  "version": "4",
  "groupKey": "{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}",
  "truncatedAlerts": 0,
  "status": "firing",
  "receiver": "default",
  "groupLabels": {
    "namespace": "monitoring"
  },
  "commonLabels": {
    "alertname": "ProbeFailure",
    "cluster": "foo",
    "namespace": "monitoring",
    "prometheus": "monitoring/k8s",
    "severity": "critical"
  },
  "commonAnnotations": null,
  "externalURL": "https://console.apps.example.com/monitoring",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "ProbeFailure",
        "cluster": "foo",
        "namespace": "monitoring",
        "node": "node.foo.example.com",
        "prometheus": "monitoring/k8s",
        "severity": "critical"
      },
      "annotations": {
        "description": "Instance https://server.example.org has been down for over 5m. Job: http_checks",
        "summary": "BlackBox Probe Failure: https://server.example.org"
      },
      "startsAt": "2023-02-06T13:08:45.828Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://console.apps.example.com/monitoring",
      "fingerprint": "1a30ba71cca2921f"
    }
  ]
}