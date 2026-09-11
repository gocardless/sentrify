# sentrify

Wraps a system command, reporting failures to Sentry.

`sentrify` runs the command you give it, streaming its output as normal. If
the command exits non-zero, the wrapper reports the failure (including
captured stderr) to Sentry, then exits `0` itself — the point is to let
Sentry carry the alert instead of the process's own exit code, which suits
wrapping cron jobs and scheduled scripts where you don't want a second,
exit-code-based alerting path racing the Sentry one.

## Installation

The primary use case is a VM downloading the binary from a GitHub release
during server configuration. Each release publishes, per `linux/amd64` and
`linux/arm64`:

- A `sentrify_<version>_linux_<arch>.tar.gz` archive containing the plain
  binary.
- A `sentrify_<version>_linux_<arch>.deb` package, installing to
  `/usr/bin/sentrify` (e.g. via a Chef recipe's `github_asset` +
  `dpkg_package` resources).

There's also a `FROM scratch` Docker image at
`ghcr.io/gocardless/sentrify`, containing just the static binary and CA
certificates. It isn't meant to be run on its own — since `sentrify` wraps a
command that has to live in the same container — but other images can pull
the binary in with a multi-stage build:

```dockerfile
COPY --from=ghcr.io/gocardless/sentrify:latest /bin/sentrify /bin/sentrify
```

## Usage

```
sentrify [--tag key=value]... [--timeout duration] [--] <command> [args...]
```

```
      --tag strings        Tags in key=value format that will be sent to Sentry
      --timeout duration   Timeout for contacting Sentry (default 10s)
```

Use `--` to mark the end of sentrify's own flags, so anything after it —
including flags — is passed straight through to the wrapped command:

```sh
sentrify --tag env=prod --timeout 5s -- /bin/bash sync-bi-data --verbose
```

## Configuration

sentrify needs a Sentry DSN to report to. It looks for one in this order:

1. The `SENTRY_DSN` environment variable.
2. A DSN compiled into the binary at build time (via `-X main.SentryDSN=...`),
   for production releases.

If neither is set, sentrify logs a warning and runs the wrapped command
without reporting failures anywhere — this is the expected behaviour in
development.

## Building

```sh
go build .
```

To stamp version metadata into the binary, matching what's attached to every
Sentry event:

```sh
go build -ldflags "\
  -X main.Version=$(git describe --tags --always) \
  -X main.Commit=$(git rev-parse HEAD) \
  -X main.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

Releases (archives, `.deb` packages, and the Docker image) are built with
[GoReleaser](https://goreleaser.com), configured in `.goreleaser.yml`. To
build everything locally without publishing:

```sh
goreleaser release --snapshot --clean
```
