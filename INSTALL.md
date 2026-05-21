# Install and test mautrix-ghdiscussions v0.1.0

## Prerequisites

- Matrix homeserver with appservice support (e.g. Synapse)
- PostgreSQL 16+
- A [GitHub App](https://github.com/settings/apps/new) you control (see main [README](README.md))
- Public HTTPS URL for webhooks (ngrok, ingress, or reverse proxy)

## 1. Pull the image

```bash
docker pull ghcr.io/trenthouliston/matrix-github-discussions:v0.1.0
```

## 2. Generate config (first run)

```bash
docker run --rm -v ghd_data:/data -p 29348:29348 \
  ghcr.io/trenthouliston/matrix-github-discussions:v0.1.0
```

This writes `/data/config.yaml` and `/data/registration.yaml` into the `ghd_data` volume, then exits.

## 3. Edit config

Mount the volume and edit `config.yaml`:

| Section | What to set |
|---------|-------------|
| `homeserver` | Your Synapse URL and server name |
| `appservice` | Generated tokens (or keep defaults from example) |
| `bridge.public_address` | Public URL, e.g. `https://bridge.example.com` |
| `database.uri` | PostgreSQL connection string |
| `network.client_id` | GitHub App Client ID |
| `network.app_id` | GitHub App ID |
| `network.webhook_secret` | GitHub App webhook secret |
| `network.app_slug` | App slug from `https://github.com/apps/<slug>` |
| `network.private_key_path` | `/data/github-app.pem` |

Copy your GitHub App private key to `/data/github-app.pem` on the volume.

## 4. Register with Synapse

Add `registration.yaml` from the volume to Synapse and restart. See [mautrix appservice docs](https://docs.mau.fi/bridges/general/registering-appservices.html).

## 5. Run the bridge

```bash
docker run -d --name ghd --restart unless-stopped \
  -v ghd_data:/data \
  -p 29348:29348 \
  ghcr.io/trenthouliston/matrix-github-discussions:v0.1.0
```

## 6. GitHub App webhook

Set the App webhook URL to:

```text
https://<bridge.public_address>/_ghdiscussions/webhook
```

## 7. Log in from Matrix

Message the bridge bot: `login` or `!gh login`, complete device flow at https://github.com/login/device.

## 8. Choose which repos to mirror

Install the GitHub App on the repositories (or org) you want:

```text
https://github.com/apps/<your-app-slug>/installations/new
```

Only discussions in those repos will be bridged. See README section **Which discussions are mirrored?**

## Helm (optional)

```bash
helm install ghd oci://ghcr.io/trenthouliston/charts/mautrix-ghdiscussions --version 0.1.0 -f my-values.yaml
```

Or from this repo: `helm install ghd ./charts/mautrix-ghdiscussions -f my-values.yaml`
