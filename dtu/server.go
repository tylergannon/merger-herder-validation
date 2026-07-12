package dtu

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func Start(config Config) (Instance, error) {
	config = withConfigDefaults(config)

	dataDir := config.DataDir
	removeDataDir := false
	if dataDir == "" {
		var err error
		dataDir, err = os.MkdirTemp("", "dtu-github-")
		if err != nil {
			return Instance{}, fmt.Errorf("create data directory: %w", err)
		}
		removeDataDir = true
	}
	keepDataDir := true
	defer func() {
		if removeDataDir && keepDataDir {
			_ = os.RemoveAll(dataDir)
		}
	}()
	if err := os.MkdirAll(filepath.Join(dataDir, "repositories"), 0o755); err != nil {
		return Instance{}, fmt.Errorf("prepare data directory: %w", err)
	}

	gitExecPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return Instance{}, fmt.Errorf("locate Git executables: %w", err)
	}
	gitBackend := filepath.Join(strings.TrimSpace(string(gitExecPath)), "git-http-backend")
	if _, err := os.Stat(gitBackend); err != nil {
		return Instance{}, fmt.Errorf("locate git-http-backend: %w", err)
	}

	publicListener, err := net.Listen("tcp", config.PublicAddress)
	if err != nil {
		return Instance{}, fmt.Errorf("listen for public API: %w", err)
	}
	controlListener, err := net.Listen("tcp", config.ControlAddress)
	if err != nil {
		publicListener.Close()
		return Instance{}, fmt.Errorf("listen for control API: %w", err)
	}

	w := &world{
		now:          config.InitialTime,
		dataDir:      dataDir,
		gitBackend:   gitBackend,
		apps:         make(map[int64]app),
		installs:     make(map[int64]installation),
		repositories: make(map[int64]repository),
		repoNames:    make(map[string]int64),
		pulls:        make(map[pullKey]pullRequest),
		tokens:       make(map[string]installationToken),
	}

	runtime := &instanceRuntime{
		publicServer:  http.Server{Handler: w.publicHandler()},
		controlServer: http.Server{Handler: w.controlHandler()},
		done:          make(chan error, 2),
		removeDataDir: removeDataDir,
		dataDir:       dataDir,
	}
	go serve(runtime.done, &runtime.publicServer, publicListener)
	go serve(runtime.done, &runtime.controlServer, controlListener)

	publicURL := url.URL{Scheme: "http", Host: publicListener.Addr().String(), Path: "/"}
	controlURL := url.URL{Scheme: "http", Host: controlListener.Addr().String(), Path: "/"}
	instance := Instance{
		GitHubURL:  publicURL,
		GitURL:     publicURL,
		ControlURL: controlURL,
		runtime:    runtime,
	}
	keepDataDir = false
	return instance, nil
}

func withConfigDefaults(config Config) Config {
	if config.PublicAddress == "" {
		config.PublicAddress = "127.0.0.1:0"
	}
	if config.ControlAddress == "" {
		config.ControlAddress = "127.0.0.1:0"
	}
	if config.InitialTime.IsZero() {
		config.InitialTime = time.Now().UTC()
	}
	return config
}

func serve(done chan<- error, server *http.Server, listener net.Listener) {
	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	done <- err
}

func (r *instanceRuntime) close(ctx context.Context) error {
	r.closeOnce.Do(func() {
		publicErr := r.publicServer.Shutdown(ctx)
		controlErr := r.controlServer.Shutdown(ctx)
		if publicErr != nil {
			_ = r.publicServer.Close()
		}
		if controlErr != nil {
			_ = r.controlServer.Close()
		}
		serveErr1 := <-r.done
		serveErr2 := <-r.done
		if r.removeDataDir {
			_ = os.RemoveAll(r.dataDir)
		}
		r.closeErr = errors.Join(publicErr, controlErr, serveErr1, serveErr2)
	})
	return r.closeErr
}
