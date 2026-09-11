package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type Options struct {
	Timeout      time.Duration
	TagKeyValues []string
}

func NewRootCommand() *cobra.Command {
	o := &Options{}
	c := &cobra.Command{
		Use:   "sentrify [flags] -- <command> [args...]",
		Short: "Wraps a system command, reporting failures to Sentry",
		Args:  cobra.MinimumNArgs(1),
		Example: `
  # Run a bash script, reporting to Sentry if it fails
  sentrify -- /bin/bash sync-bi-data`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          o.Run,
	}

	c.Flags().DurationVar(&o.Timeout, "timeout", 10*time.Second, "Timeout for contacting Sentry")
	c.Flags().StringSliceVar(&o.TagKeyValues, "tag", nil, "Tags in key=value format that will be sent to Sentry")

	return c
}

func (o *Options) Run(_ *cobra.Command, args []string) error {
	return CaptureSentry(o.Timeout, func() error {
		return o.run(args)
	})
}

// run executes the wrapped command, streaming its stderr through to our own
// while also capturing it so it can be attached to any Sentry report. A
// non-zero exit from the wrapped command is reported to Sentry, but is not
// itself treated as a sentrify failure: the whole point of sentrify is to
// let Sentry, rather than the exit code, carry the alert.
func (o *Options) run(args []string) error {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return WrapUserError(fmt.Errorf("executable not found: %w", err))
	}

	cmd := exec.Command(path, args[1:]...)
	cmd.Stdout = os.Stdout

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return WrapUserError(fmt.Errorf("failed to start command: %w", err))
	}

	stderrOutput, err := io.ReadAll(io.TeeReader(stderr, os.Stderr))
	if err != nil {
		return fmt.Errorf("failed to read from stderr: %w", err)
	}

	switch waitErr := cmd.Wait(); waitErr.(type) {
	case nil:
		logger.Info("Successfully ran command")
	case *exec.ExitError:
		logger.With(zap.Error(waitErr)).Error("Command failed, notifying Sentry...")
		o.reportFailure(args, stderrOutput)
	default:
		return fmt.Errorf("unexpected error running command: %w", waitErr)
	}

	return nil
}

func (o *Options) reportFailure(args []string, stderrOutput []byte) {
	if dsn() == "" {
		logger.Warn("No SENTRY_DSN configured, this failure will go unreported")
		return
	}

	var eventID *sentry.EventID
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTags(o.tags())
		scope.SetContext("stderr", sentry.Context{"output": string(stderrOutput)})
		eventID = sentry.CaptureMessage(fmt.Sprintf("sentrify: %s", strings.Join(args, " ")))
	})

	if !sentry.Flush(o.Timeout) {
		logger.Warn("Timed out contacting Sentry, this error will go unreported")
		return
	}

	if eventID != nil {
		logger.With(zap.String("eventID", string(*eventID))).Info("Reported failure to Sentry")
	}
}

func (o *Options) tags() map[string]string {
	tags := sentryContext()
	for _, keyValue := range o.TagKeyValues {
		pair := strings.SplitN(keyValue, "=", 2)
		if len(pair) == 2 {
			tags[pair[0]] = pair[1]
		} else {
			logger.With(zap.String("tag", keyValue)).Warn("Failed to parse key=value tag")
		}
	}

	return tags
}
