package main

import "runtime"

// These are injected at build time via -ldflags. SentryDSN is only set in
// production releases; if empty, sentrify falls back to the SENTRY_DSN
// environment variable.
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = runtime.Version()
	SentryDSN = ""
)
