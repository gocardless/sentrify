package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// UserError denotes an error caused by bad user behaviour (e.g. a typo'd
// command). Errors marked this way are excluded from Sentry reporting.
type UserError struct{ error }

func (e UserError) UserError() bool { return true }

// WrapUserError marks the given error as having been caused by bad user
// behaviour, so CaptureSentry will not report it.
func WrapUserError(err error) error {
	if err == nil {
		return nil
	}

	return UserError{err}
}

// ignoreError defines the errors we should not send to Sentry: transient
// network timeouts, and anything caused by user configuration.
func ignoreError(err error) bool {
	if nerr, ok := err.(net.Error); ok {
		return nerr.Timeout()
	}

	if uerr, ok := err.(interface{ UserError() bool }); ok {
		return uerr.UserError()
	}

	return false
}

// sentryContext returns the set of tags attached to every event sentrify
// sends to Sentry.
func sentryContext() map[string]string {
	return map[string]string{
		"args":       strings.Join(os.Args, " "),
		"user":       os.Getenv("USER"),
		"host":       os.Getenv("HOST"),
		"version":    Version,
		"commit":     Commit,
		"build_date": Date,
		"go_version": GoVersion,
	}
}

// dsn resolves the Sentry DSN to use: an explicit SENTRY_DSN environment
// variable takes precedence over the value compiled into the binary.
func dsn() string {
	if env := os.Getenv("SENTRY_DSN"); env != "" {
		return env
	}

	return SentryDSN
}

// CaptureSentry initialises the Sentry SDK (if a DSN is configured) and runs
// the given function, recovering from any panic. Errors and panics that
// aren't ignorable are reported to Sentry before being returned to the
// caller. If no DSN is configured we simply run the function uninstrumented,
// which should only happen in development.
func CaptureSentry(timeout time.Duration, run func() error) (err error) {
	d := dsn()
	if d == "" {
		logger.Warn("No SENTRY_DSN configured, continuing without exception tracking...")
		return run()
	}

	if initErr := sentry.Init(sentry.ClientOptions{Dsn: d}); initErr != nil {
		logger.With(zap.Error(initErr)).Error("Failed to initialise Sentry client, check your DSN")
		return run()
	}

	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(timeout)
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	if err = run(); err != nil && !ignoreError(err) {
		sentry.CaptureException(err)
		sentry.Flush(timeout)
	}

	return err
}
