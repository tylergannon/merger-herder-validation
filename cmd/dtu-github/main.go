package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tylergannon/merger-herder-validation/dtu"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	publicAddress := flag.String("public-address", "127.0.0.1:0", "public GitHub and Git listen address")
	controlAddress := flag.String("control-address", "127.0.0.1:0", "private control listen address")
	dataDir := flag.String("data-dir", "", "state directory; temporary when omitted")
	initialTime := flag.String("initial-time", "", "initial RFC3339 virtual time")
	flag.Parse()

	now := time.Now().UTC()
	if *initialTime != "" {
		parsed, err := time.Parse(time.RFC3339, *initialTime)
		if err != nil {
			return fmt.Errorf("parse initial time: %w", err)
		}
		now = parsed
	}
	instance, err := dtu.Start(dtu.Config{
		PublicAddress:  *publicAddress,
		ControlAddress: *controlAddress,
		DataDir:        *dataDir,
		InitialTime:    now,
	})
	if err != nil {
		return err
	}

	ready := struct {
		GitHubURL  string `json:"github_url"`
		GitURL     string `json:"git_url"`
		ControlURL string `json:"control_url"`
	}{
		GitHubURL:  instance.GitHubURL.String(),
		GitURL:     instance.GitURL.String(),
		ControlURL: instance.ControlURL.String(),
	}
	if err := json.NewEncoder(os.Stdout).Encode(ready); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return instance.Close(shutdown)
}
