package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	var err error
	if logger, err = zap.NewProduction(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise logger: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
