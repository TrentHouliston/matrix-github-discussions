# Helm chart: mautrix-ghdiscussions

Deploy the GitHub Discussions Matrix bridge on Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3
- PostgreSQL 16+ (external or in-cluster)
- Matrix homeserver with appservice support
- GitHub App configured (see repository [README](../../README.md))

## Install from Git

```bash
helm install ghd ./charts/mautrix-ghdiscussions \
  --namespace matrix --create-namespace \
  -f my-values.yaml
```

## Install from OCI (after a release)

```bash
helm install ghd oci://ghcr.io/trenthouliston/charts/mautrix-ghdiscussions \
  --version 0.1.0 \
  -f my-values.yaml
```

## Minimal values

Create `my-values.yaml`:

```yaml
image:
  repository: ghcr.io/trenthouliston/matrix-github-discussions
  tag: latest

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: bridge.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: bridge-tls
      hosts:
        - bridge.example.com

config:
  homeserver:
    address: https://matrix.example.com
    domain: example.com
  bridge:
    public_address: https://bridge.example.com
  database:
    uri: postgres://user:pass@postgres.matrix.svc:5432/mautrix_ghdiscussions?sslmode=disable
  network:
    client_id: "Iv1...."
    app_id: 123456
    webhook_secret: "your-webhook-secret"
    app_slug: your-app-slug

appservice:
  # Generate once: openssl rand -hex 32
  as_token: "..."
  hs_token: "..."

githubApp:
  privateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
```

## First-time registration

1. Install the chart; the pod may exit once after writing `registration.yaml` to the persistent volume.
2. Read registration: `kubectl exec deploy/ghd-mautrix-ghdiscussions -- cat /data/registration.yaml`
3. Add it to your homeserver config and restart Synapse.
4. Restart the bridge deployment.

## GitHub webhook

Point the GitHub App webhook to:

```text
https://<config.bridge.public_address>/_ghdiscussions/webhook
```

This must match the ingress hostname when using Kubernetes.

## Upgrading

```bash
helm upgrade ghd ./charts/mautrix-ghdiscussions -f my-values.yaml
```

## Uninstall

```bash
helm uninstall ghd
```
