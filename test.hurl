POST http://localhost:8080/webhook
User-Agent: Alertmanager/0.25.0
Accept:
{
  "receiver": "default",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "ProbeFailure",
        "cluster": "pia2",
        "namespace": "openshift-monitoring",
        "node": "node.foo.ch",
        "openshift_io_alert_source": "platform",
        "prometheus": "openshift-monitoring/k8s",
        "severity": "critical"
      },
      "annotations": {
        "description": "Instance https://server.example.org has been down for over 5m. Job: http_checks",
        "summary": "BlackBox Probe Failure: https://server.example.org"
      },
      "startsAt": "2023-02-06T13:08:45.828Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://console.apps.ioipioipo/monitoring....",
      "fingerprint": "1a30ba71cca2921f"
    }
  ],
  "groupLabels": {
    "namespace": "openshift-monitoring"
  },
  "commonLabels": {
    "alertname": "ProbeFailure",
    "cluster": "pta2",
    "namespace": "openshift-monitoring",
    "openshift_io_alert_source": "platform",
    "prometheus": "openshift-monitoring/k8s",
    "severity": "critical"
  },
  "commonAnnotations": {},
  "externalURL": "https://console.apps.joihjoijiooij/monitoring",
  "version": "4",
  "groupKey": "{}/{severity=\"critical\"}:{alertname=\"ProbeFailure\"}",
  "truncatedAlerts": 0
}