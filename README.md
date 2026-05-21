# mautrix-ghdiscussions

A Matrix puppet bridge for [GitHub Discussions](https://github.com/features/discussions), built on [mautrix-go bridgev2](https://github.com/mautrix/go).

Each GitHub Discussion becomes a Matrix room. Top-level comments are regular messages; replies to comments are bridged as Matrix threads. Reactions, edits, and deletes are supported (reactions are polled from GitHub because webhooks do not include them).

## Features

- **Double puppeting** via GitHub App OAuth device flow (`ghu_*` user-to-server tokens)
- **One Matrix room per Discussion**
- **Bidirectional** comments, edits, deletes, and reactions (GitHub emoji set)
- **Extensible** portal metadata for future Issue/PR bridging

## Requirements

- Go 1.25+
- PostgreSQL 16+
- A Matrix homeserver with application service support
- [libolm](https://gitlab.com/matrix-org/olm) (for end-to-bridge encryption)
- A GitHub App you control

## GitHub App setup

1. Create a new GitHub App at https://github.com/settings/apps/new
2. **Permissions** (repository):
   - Discussions: Read & write
   - Metadata: Read-only
3. **Subscribe to events**:
   - Discussion
   - Discussion comment
   - Installation
   - Installation repositories
4. Enable **Device flow** under "Identifying and authorizing users"
5. Set the **Webhook URL** to `https://<bridge-public-url>/_ghdiscussions/webhook`
6. Generate a webhook secret and download the private key
7. Note the **App ID** and **Client ID**

## Bridge setup

```bash
go build -o mautrix-ghdiscussions ./cmd/mautrix-ghdiscussions/
./mautrix-ghdiscussions -g -c config.yaml -r registration.yaml
```

Edit `config.yaml`:

- Configure `homeserver`, `appservice`, and `database` as usual for mautrix bridges
- Set `bridge.public_address` to your bridge's public HTTPS URL
- Under `network:` set `client_id`, `app_id`, `private_key_path`, `webhook_secret`, and `app_slug`

Start the bridge and register the appservice on your homeserver.

## Usage

1. In Matrix, message the bridge bot: `login` (or `!gh login` depending on command prefix)
2. Complete the GitHub device flow (visit https://github.com/login/device and enter the code)
3. Install the GitHub App on repositories you want bridged
4. New discussion activity in those repos will create/use Matrix portal rooms

## Which discussions are mirrored?

There is **no per-discussion picker** in v1. Scope is controlled entirely by **where you install the GitHub App**:

| You control | How |
|-------------|-----|
| **Repositories** | Install the bridge's GitHub App on specific repos (or an org). Only those repos are tracked in the `github_installation` table. |
| **Your Matrix account** | Each Matrix user logs in separately; webhooks are routed to the GitHub user who installed the App on that repo. |
| **What gets a room** | Each **Discussion** in a tracked repo gets its own Matrix portal room when GitHub sends a `discussion` / `discussion_comment` webhook (typically when the discussion is created or someone comments). |

What is **not** supported yet:

- Choosing individual discussions or categories (e.g. only "Q&A")
- Bridging old discussions that had no activity since install (no backfill of history unless you add comments)
- Starting new discussions from Matrix

To stop mirroring a repo, remove the GitHub App installation from that repository (or org) in GitHub settings.

## Quick test (Docker)

```bash
docker pull ghcr.io/trenthouliston/matrix-github-discussions:latest

# First run writes example config + registration to the volume, then exits.
docker run --rm -v ghd_data:/data -p 29348:29348 \
  ghcr.io/trenthouliston/matrix-github-discussions:latest

# Edit /data/config.yaml and /data/registration.yaml on the volume, register with Synapse, then:
docker run -d --name ghd -v ghd_data:/data -p 29348:29348 \
  ghcr.io/trenthouliston/matrix-github-discussions:latest
```

See [releases](https://github.com/TrentHouliston/matrix-github-discussions/releases) for versioned images, binaries, and the Helm chart.

## Architecture

```
GitHub Discussions ──webhook──► Bridge ──AS API──► Matrix
       ▲                           │
       └── GraphQL (ghu_* token) ◄─┘
```

- **Inbound**: `discussion` / `discussion_comment` webhooks → ghost users + portal rooms
- **Outbound**: Matrix messages → `addDiscussionComment` GraphQL mutation
- **Reactions**: Polled every `reaction_poll_interval_seconds` on recently active portals

## Caveats

- GitHub Discussions webhooks are marked **public preview** and may change
- Reactions are **eventually consistent** (poll interval, default 60s)
- Each user must **install the App** on private repos they want bridged
- User tokens expire after ~8 hours; the bridge refreshes them automatically
- Starting new Discussions from Matrix is not supported in v1

## Docker

```bash
docker build -t mautrix-ghdiscussions:local .
docker run --rm -it -v mautrix_ghd_data:/data -p 29348:29348 mautrix-ghdiscussions:local
```

Or use Compose (includes PostgreSQL):

```bash
docker compose up --build
```

Published images (on `main` and version tags): `ghcr.io/trenthouliston/matrix-github-discussions`

## Kubernetes (Helm)

See [charts/mautrix-ghdiscussions/README.md](charts/mautrix-ghdiscussions/README.md).

```bash
helm install ghd ./charts/mautrix-ghdiscussions -f my-values.yaml
```

After tagging a release (`v0.1.0`), the chart is also published to:

```bash
helm install ghd oci://ghcr.io/trenthouliston/charts/mautrix-ghdiscussions --version 0.1.0
```

Copy [charts/mautrix-ghdiscussions/values-production.example.yaml](charts/mautrix-ghdiscussions/values-production.example.yaml) as a starting point.

## CI and releases

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `go.yml` | push / PR | Tests, pre-commit, Helm lint |
| `docker.yml` | push / PR / tags | Multi-arch image to GHCR |
| `release.yml` | tags `v*` | Goreleaser binaries + Helm chart OCI + GitHub Release |

Create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Development

```bash
go test ./pkg/connector/...
./build.sh
pre-commit run --all-files   # optional
```

Building the full binary requires libolm headers (`brew install libolm` on macOS).

## License

Mozilla Public License 2.0 (same as mautrix-go)
