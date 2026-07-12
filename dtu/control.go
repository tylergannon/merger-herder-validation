package dtu

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (w *world) controlHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /apps", w.createApp)
	mux.HandleFunc("POST /installations", w.createInstallation)
	mux.HandleFunc("POST /repositories", w.createRepository)
	mux.HandleFunc("POST /pulls", w.createPullRequest)
	mux.HandleFunc("POST /pulls/state", w.changePullRequestState)
	mux.HandleFunc("POST /clock/advance", w.advanceTime)
	mux.HandleFunc("POST /workflows", w.configureWorkflow)
	mux.HandleFunc("POST /deliveries", w.deliverEvent)
	mux.HandleFunc("POST /events/duplicate", w.duplicateEvent)
	mux.HandleFunc("POST /workflow-runs/transition", w.transitionWorkflowRun)
	mux.HandleFunc("GET /state", w.getState)
	return mux
}

func (w *world) createApp(response http.ResponseWriter, request *http.Request) {
	var input AppInput
	if !decodeControl(response, request, &input) {
		return
	}
	block, _ := pem.Decode([]byte(input.PublicKeyPEM))
	if block == nil {
		writeControlError(response, http.StatusBadRequest, "invalid public key PEM")
		return
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		writeControlError(response, http.StatusBadRequest, "invalid public key")
		return
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		writeControlError(response, http.StatusBadRequest, "public key is not RSA")
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if input.ID <= 0 || w.apps[input.ID].id != 0 {
		writeControlError(response, http.StatusConflict, "invalid or duplicate App ID")
		return
	}
	w.apps[input.ID] = app{
		id:            input.ID,
		publicKey:     key,
		webhookURL:    input.WebhookURL,
		webhookSecret: input.WebhookSecret,
	}
	w.mutations++
	writeJSON(response, http.StatusCreated, map[string]int64{"id": input.ID})
}

func (w *world) createInstallation(response http.ResponseWriter, request *http.Request) {
	var input InstallationInput
	if !decodeControl(response, request, &input) {
		return
	}
	for permission, level := range input.Permissions {
		if permission == "" || permissionRank(level) == 0 {
			writeControlError(response, http.StatusBadRequest, "invalid permission")
			return
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if input.ID <= 0 || w.installs[input.ID].id != 0 {
		writeControlError(response, http.StatusConflict, "invalid or duplicate installation ID")
		return
	}
	if _, found := w.apps[input.AppID]; !found {
		writeControlError(response, http.StatusBadRequest, "unknown App")
		return
	}
	w.installs[input.ID] = installation{
		id:            input.ID,
		appID:         input.AppID,
		active:        input.Active,
		permissions:   copyMap(input.Permissions),
		repositoryIDs: make(map[int64]struct{}),
	}
	w.mutations++
	writeJSON(response, http.StatusCreated, map[string]int64{"id": input.ID})
}

func (w *world) createRepository(response http.ResponseWriter, request *http.Request) {
	var input RepositoryInput
	if !decodeControl(response, request, &input) {
		return
	}
	if input.ID <= 0 || !validPathComponent(input.Owner) || !validPathComponent(input.Name) {
		writeControlError(response, http.StatusBadRequest, "invalid repository")
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	installation, found := w.installs[input.InstallationID]
	if !found {
		writeControlError(response, http.StatusBadRequest, "unknown installation")
		return
	}
	if _, found := w.repositories[input.ID]; found {
		writeControlError(response, http.StatusConflict, "duplicate repository ID")
		return
	}
	key := repoName(input.Owner, input.Name)
	if _, found := w.repoNames[key]; found {
		writeControlError(response, http.StatusConflict, "duplicate repository name")
		return
	}
	for id := range installation.repositoryIDs {
		if strings.EqualFold(w.repositories[id].name, input.Name) {
			writeControlError(response, http.StatusConflict, "duplicate repository name in installation")
			return
		}
	}

	gitDir := filepath.Join(w.dataDir, "repositories", input.Owner, input.Name+".git")
	if err := runGit("", "init", "--bare", gitDir); err != nil {
		writeControlError(response, http.StatusInternalServerError, err.Error())
		return
	}
	if err := runGit(gitDir, "config", "http.receivepack", "true"); err != nil {
		_ = os.RemoveAll(gitDir)
		writeControlError(response, http.StatusInternalServerError, err.Error())
		return
	}

	w.repositories[input.ID] = repository{
		id:             input.ID,
		owner:          input.Owner,
		name:           input.Name,
		installationID: input.InstallationID,
		gitDir:         gitDir,
	}
	w.receiveLocks[input.ID] = new(sync.Mutex)
	w.repoNames[key] = input.ID
	installation.repositoryIDs[input.ID] = struct{}{}
	w.installs[input.InstallationID] = installation
	w.mutations++
	writeJSON(response, http.StatusCreated, map[string]int64{"id": input.ID})
}

func validPathComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "/\\")
}

func (w *world) createPullRequest(response http.ResponseWriter, request *http.Request) {
	var input PullRequestInput
	if !decodeControl(response, request, &input) {
		return
	}
	if input.State == "" {
		input.State = "open"
	}
	if input.Number <= 0 || input.BaseRef == "" || input.HeadRef == "" || (input.State != "open" && input.State != "closed") {
		writeControlError(response, http.StatusBadRequest, "invalid pull request")
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	repository, found := w.repositories[input.RepositoryID]
	if !found {
		writeControlError(response, http.StatusBadRequest, "unknown repository")
		return
	}
	key := pullKey{repositoryID: input.RepositoryID, number: input.Number}
	if _, found := w.pulls[key]; found {
		writeControlError(response, http.StatusConflict, "duplicate pull request")
		return
	}
	baseSHA := resolveRef(repository.gitDir, input.BaseRef)
	headSHA := resolveRef(repository.gitDir, input.HeadRef)
	if baseSHA == "" || headSHA == "" {
		writeControlError(response, http.StatusBadRequest, "base and head refs must exist")
		return
	}
	w.pulls[key] = pullRequest{
		repositoryID: input.RepositoryID,
		number:       input.Number,
		baseRef:      input.BaseRef,
		headRef:      input.HeadRef,
		baseSHA:      baseSHA,
		headSHA:      headSHA,
		state:        input.State,
		draft:        input.Draft,
	}
	w.mutations++
	writeJSON(response, http.StatusCreated, map[string]int{"number": input.Number})
}

func (w *world) changePullRequestState(response http.ResponseWriter, request *http.Request) {
	var input PullRequestStateInput
	if !decodeControl(response, request, &input) {
		return
	}
	if input.State != "open" && input.State != "closed" {
		writeControlError(response, http.StatusBadRequest, "invalid pull request state")
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	key := pullKey{repositoryID: input.RepositoryID, number: input.Number}
	pull, found := w.pulls[key]
	if !found {
		writeControlError(response, http.StatusNotFound, "unknown pull request")
		return
	}
	pull.state = input.State
	w.pulls[key] = pull
	w.mutations++
	writeJSON(response, http.StatusOK, map[string]string{"state": input.State})
}

func (w *world) advanceTime(response http.ResponseWriter, request *http.Request) {
	var input AdvanceTimeInput
	if !decodeControl(response, request, &input) {
		return
	}
	duration, err := time.ParseDuration(input.Duration)
	if err != nil || duration < 0 {
		writeControlError(response, http.StatusBadRequest, "invalid duration")
		return
	}
	w.mu.Lock()
	w.now = w.now.Add(duration)
	w.mutations++
	now := w.now
	w.mu.Unlock()
	writeJSON(response, http.StatusOK, map[string]time.Time{"now": now})
}

func (w *world) configureWorkflow(response http.ResponseWriter, request *http.Request) {
	var input WorkflowInput
	if !decodeControl(response, request, &input) {
		return
	}
	if input.ID <= 0 || input.Name == "" || input.Path == "" || input.ReleaseRef == "" {
		writeControlError(response, http.StatusBadRequest, "invalid workflow")
		return
	}
	if err := runGit("", "check-ref-format", "--branch", input.ReleaseRef); err != nil {
		writeControlError(response, http.StatusBadRequest, "invalid release ref")
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, found := w.repositories[input.RepositoryID]; !found {
		writeControlError(response, http.StatusBadRequest, "unknown repository")
		return
	}
	w.workflows[input.RepositoryID] = workflowConfig{
		repositoryID: input.RepositoryID,
		id:           input.ID,
		name:         input.Name,
		path:         input.Path,
		releaseRef:   input.ReleaseRef,
	}
	w.mutations++
	writeJSON(response, http.StatusCreated, map[string]int64{"id": input.ID})
}

func (w *world) getState(response http.ResponseWriter, _ *http.Request) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	unsupported := append([]UnsupportedRequest(nil), w.unsupported...)
	pendingEvents := append([]PendingEvent(nil), w.pendingEvents...)
	workflowRuns := append([]WorkflowRun(nil), w.workflowRuns...)
	observationErrors := append([]ObservationError(nil), w.observationErrors...)
	deliveryAttempts := append([]DeliveryAttempt(nil), w.deliveryAttempts...)
	writeJSON(response, http.StatusOK, StateSnapshot{
		Now:                 w.now,
		Apps:                len(w.apps),
		Installations:       len(w.installs),
		Repositories:        len(w.repositories),
		PullRequests:        len(w.pulls),
		Tokens:              len(w.tokens),
		Mutations:           w.mutations,
		UnsupportedRequests: unsupported,
		PendingEvents:       pendingEvents,
		WorkflowRuns:        workflowRuns,
		ObservationErrors:   observationErrors,
		DeliveryAttempts:    deliveryAttempts,
	})
}

func decodeControl(response http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeControlError(response, http.StatusBadRequest, "invalid JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeControlError(response, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}

func writeControlError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"message": message})
}

func runGit(dir string, arguments ...string) error {
	command := exec.Command("git", arguments...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
