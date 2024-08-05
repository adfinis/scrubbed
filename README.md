# Alertmanager Webhook Content Scrubber

This repository creates and publishes Docker image for deployment of Alertmanager filtering proxy.

This proxy is useful for preventing sensitive information (e.g. IP addressess, hostnames, alert descriptions, etc.) leaving organisational boundaries when monitoring is outsourced to external entity.

For convenience, Dockerfile and deployment to couple filtering proxy with Signalilo is also provided.

## Installation

See `deploy/kustomize` for Kustomize based deployment.

## Configuration

Patch ConfigMaps using Kustomize overlay. Examples provided in `deploy/kustomize/overlays`.

### Proxy

Implicitly uses default HTTP_PROXY, HTTPS_PROXY and NO_PROXY environment variables.

### Alertmanager

Add receiver to Alertmanager configuration:

```yaml
receivers:
  - name: Default
    webhook_configs:
      - url: >-
          http://scrubbed.scrubbed.svc.cluster.local:8080/webhook
        send_resolved: true
        http_config:
          bearer_token: "foo"
```

## Development

### Release Management

The CI/CD setup uses semantic commit messages following the
[conventional commits standard](https://www.conventionalcommits.org/en/v1.0.0/).
There is a GitHub Action in [.github/workflows/semantic-release.yaml](./.github/workflows/semantic-release.yaml)
that uses [go-semantic-commit](https://go-semantic-release.xyz/) to create new releases.

The commit message should be structured as follows:

```console
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

The commit contains the following structural elements, to communicate intent to the consumers of your library:

1. **fix:** a commit of the type `fix` patches gets released with a PATCH version bump
1. **feat:** a commit of the type `feat` gets released as a MINOR version bump
1. **BREAKING CHANGE:** a commit that has a footer `BREAKING CHANGE:` gets released as a MAJOR version bump
1. types other than `fix:` and `feat:` are allowed and don't trigger a release

If a commit does not contain a conventional commit style message you can fix
it during the squash and merge operation on the PR.

## References

* https://github.com/vshn/signalilo
* https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
