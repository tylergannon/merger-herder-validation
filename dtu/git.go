package dtu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const zeroObjectID = "0000000000000000000000000000000000000000"

func (w *world) serveGit(response http.ResponseWriter, request *http.Request, parts []string) {
	service, methodOK := gitService(request, parts)
	if !methodOK {
		w.handleUnsupported(response, request)
		return
	}
	username, rawToken, authOK := request.BasicAuth()
	if !authOK || username != "x-access-token" {
		response.Header().Set("WWW-Authenticate", `Basic realm="GitHub"`)
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}

	w.mu.RLock()
	token, found := w.tokens[rawToken]
	now := w.now
	repositoryID, repoFound := w.repoNames[repoName(parts[0], strings.TrimSuffix(parts[1], ".git"))]
	repository := w.repositories[repositoryID]
	receiveLock := w.receiveLocks[repositoryID]
	w.mu.RUnlock()
	if !found || !now.Before(token.expiresAt) {
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}
	if !repoFound {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if service == "git-receive-pack" && permissionRank(token.permissions["contents"]) < permissionRank("write") {
		writeError(response, http.StatusForbidden, "Resource not accessible by integration")
		return
	}
	if service == "git-upload-pack" && permissionRank(token.permissions["contents"]) < permissionRank("read") {
		writeError(response, http.StatusForbidden, "Resource not accessible by integration")
		return
	}

	root := filepath.Join(w.dataDir, "repositories")
	receivePack := request.Method == http.MethodPost && service == "git-receive-pack"
	if receivePack {
		receiveLock.Lock()
		defer receiveLock.Unlock()
	}
	beforeRefs := map[string]string{}
	if receivePack {
		var err error
		beforeRefs, err = branchRefs(repository.gitDir)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "Unable to inspect repository refs")
			return
		}
	}
	handler := cgi.Handler{
		Path: w.gitBackend,
		Root: "/",
		Dir:  root,
		Env: []string{
			"GIT_PROJECT_ROOT=" + root,
			"GIT_HTTP_EXPORT_ALL=1",
		},
		InheritEnv: []string{"PATH"},
		Stderr:     os.Stderr,
	}
	canonicalRequest := request.Clone(request.Context())
	canonicalURL := *request.URL
	canonicalURL.Path = "/" + repository.owner + "/" + repository.name + ".git/" + strings.Join(parts[2:], "/")
	canonicalRequest.URL = new(canonicalURL)
	target := response
	buffer := responseBuffer{header: make(http.Header)}
	if receivePack {
		target = &buffer
	}
	handler.ServeHTTP(target, canonicalRequest)
	if receivePack {
		afterRefs, err := branchRefs(repository.gitDir)
		if err != nil {
			w.recordObservationError(repository.id, "post-receive ref snapshot", err)
			writeError(response, http.StatusInternalServerError, "Unable to confirm repository refs")
			return
		}
		w.recordRefChanges(repository, token, beforeRefs, afterRefs)
		w.refreshPullSnapshots(repository)
		buffer.flush(response)
	}
}

func gitService(request *http.Request, parts []string) (string, bool) {
	if len(parts) == 4 && parts[2] == "info" && parts[3] == "refs" && request.Method == http.MethodGet {
		service := request.URL.Query().Get("service")
		return service, service == "git-upload-pack" || service == "git-receive-pack"
	}
	if len(parts) == 3 && request.Method == http.MethodPost {
		return parts[2], parts[2] == "git-upload-pack" || parts[2] == "git-receive-pack"
	}
	return "", false
}

func resolveRef(gitDir, ref string) string {
	command := exec.Command("git", "--git-dir", gitDir, "rev-parse", "--verify", "refs/heads/"+ref+"^{commit}")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func branchRefs(gitDir string) (map[string]string, error) {
	command := exec.Command("git", "--git-dir", gitDir, "for-each-ref", "--format=%(refname:strip=2) %(objectname)", "refs/heads")
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	refs := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			refs[fields[0]] = fields[1]
		}
	}
	return refs, nil
}

func (w *world) recordRefChanges(repository repository, token installationToken, before, after map[string]string) {
	refSet := make(map[string]struct{}, len(before)+len(after))
	for ref := range before {
		refSet[ref] = struct{}{}
	}
	for ref := range after {
		refSet[ref] = struct{}{}
	}
	refs := make([]string, 0, len(refSet))
	for ref := range refSet {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	forced := make(map[string]bool, len(refs))
	for _, ref := range refs {
		forced[ref] = forcedUpdate(repository.gitDir, before[ref], after[ref])
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	changed := false
	for _, ref := range refs {
		beforeSHA := before[ref]
		afterSHA := after[ref]
		if beforeSHA == afterSHA {
			continue
		}
		changed = true
		installation := w.installs[token.installationID]
		w.appendPushEvent(repository, installation.appID, ref, beforeSHA, afterSHA, forced[ref])
		workflow, configured := w.workflows[repository.id]
		if configured && workflow.releaseRef == ref && afterSHA != "" {
			w.appendQueuedWorkflow(repository, workflow, ref, afterSHA)
		}
	}
	if changed {
		w.mutations++
	}
}

func (w *world) appendPushEvent(repository repository, appID int64, ref, beforeSHA, afterSHA string, forced bool) {
	if beforeSHA == "" {
		beforeSHA = zeroObjectID
	}
	if afterSHA == "" {
		afterSHA = zeroObjectID
	}
	payload := struct {
		Ref          string            `json:"ref"`
		Before       string            `json:"before"`
		After        string            `json:"after"`
		Created      bool              `json:"created"`
		Deleted      bool              `json:"deleted"`
		Forced       bool              `json:"forced"`
		Repository   eventRepository   `json:"repository"`
		Installation eventInstallation `json:"installation"`
		Sender       eventSender       `json:"sender"`
	}{
		Ref:          "refs/heads/" + ref,
		Before:       beforeSHA,
		After:        afterSHA,
		Created:      beforeSHA == zeroObjectID,
		Deleted:      afterSHA == zeroObjectID,
		Forced:       forced,
		Repository:   eventRepository{ID: repository.id, FullName: repository.owner + "/" + repository.name},
		Installation: eventInstallation{ID: repository.installationID},
		Sender:       eventSender{ID: appID, Login: fmt.Sprintf("dtu-app-%d[bot]", appID), Type: "Bot"},
	}
	w.appendPendingEvent("push", "", repository.id, payload)
}

func (w *world) appendQueuedWorkflow(repository repository, workflow workflowConfig, ref, sha string) {
	w.nextRunID++
	run := WorkflowRun{
		ID:           w.nextRunID,
		Attempt:      1,
		RepositoryID: repository.id,
		WorkflowID:   workflow.id,
		WorkflowName: workflow.name,
		WorkflowPath: workflow.path,
		Event:        "push",
		HeadBranch:   ref,
		HeadSHA:      sha,
		Status:       "queued",
	}
	w.workflowRuns = append(w.workflowRuns, run)
	w.appendWorkflowEvent(repository, run, "requested")
}

func (w *world) appendWorkflowEvent(repository repository, run WorkflowRun, action string) {
	var conclusion any
	if run.Conclusion != "" {
		conclusion = run.Conclusion
	}
	payload := struct {
		Action       string            `json:"action"`
		WorkflowRun  workflowRunEvent  `json:"workflow_run"`
		Repository   eventRepository   `json:"repository"`
		Installation eventInstallation `json:"installation"`
	}{
		Action: action,
		WorkflowRun: workflowRunEvent{
			ID: run.ID, RunAttempt: run.Attempt, WorkflowID: run.WorkflowID,
			Name: run.WorkflowName, Path: run.WorkflowPath, Event: run.Event,
			HeadBranch: run.HeadBranch, HeadSHA: run.HeadSHA, Status: run.Status,
			Conclusion: conclusion,
		},
		Repository:   eventRepository{ID: repository.id, FullName: repository.owner + "/" + repository.name},
		Installation: eventInstallation{ID: repository.installationID},
	}
	w.appendPendingEvent("workflow_run", action, repository.id, payload)
}

func (w *world) appendPendingEvent(event, action string, repositoryID int64, payload any) {
	repository := w.repositories[repositoryID]
	installation := w.installs[repository.installationID]
	w.appendPendingAppEvent(event, action, installation.appID, repositoryID, payload)
}

func (w *world) appendPendingAppEvent(event, action string, appID, repositoryID int64, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("marshal pending %s event: %v", event, err))
	}
	w.nextEventID++
	w.pendingEvents = append(w.pendingEvents, PendingEvent{
		GUID:         fmt.Sprintf("dtu-%012d", w.nextEventID),
		Event:        event,
		Action:       action,
		AppID:        appID,
		RepositoryID: repositoryID,
		CreatedAt:    w.now,
		Body:         body,
	})
}

type eventRepository struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

type eventInstallation struct {
	ID int64 `json:"id"`
}

type eventSender struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type workflowRunEvent struct {
	ID         int64  `json:"id"`
	RunAttempt int    `json:"run_attempt"`
	WorkflowID int64  `json:"workflow_id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion any    `json:"conclusion"`
}

type responseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *responseBuffer) Header() http.Header {
	return b.header
}

func (b *responseBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *responseBuffer) Write(value []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(value)
}

func (b *responseBuffer) flush(response http.ResponseWriter) {
	for key, values := range b.header {
		for _, value := range values {
			response.Header().Add(key, value)
		}
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	response.WriteHeader(status)
	_, _ = response.Write(b.body.Bytes())
}

func forcedUpdate(gitDir, beforeSHA, afterSHA string) bool {
	if beforeSHA == "" || afterSHA == "" {
		return false
	}
	command := exec.Command("git", "--git-dir", gitDir, "merge-base", "--is-ancestor", beforeSHA, afterSHA)
	return command.Run() != nil
}

func (w *world) recordObservationError(repositoryID int64, operation string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.observationErrors = append(w.observationErrors, ObservationError{
		RepositoryID: repositoryID,
		Operation:    operation,
		Message:      err.Error(),
		At:           w.now,
	})
}

func (w *world) refreshPullSnapshots(repository repository) {
	type refSnapshot struct {
		key     pullKey
		baseRef string
		headRef string
	}
	w.mu.RLock()
	snapshots := make([]refSnapshot, 0)
	for key, pull := range w.pulls {
		if pull.repositoryID == repository.id {
			snapshots = append(snapshots, refSnapshot{key: key, baseRef: pull.baseRef, headRef: pull.headRef})
		}
	}
	w.mu.RUnlock()

	for _, snapshot := range snapshots {
		baseSHA := resolveRef(repository.gitDir, snapshot.baseRef)
		headSHA := resolveRef(repository.gitDir, snapshot.headRef)
		w.mu.Lock()
		pull, found := w.pulls[snapshot.key]
		if found && pull.baseRef == snapshot.baseRef && pull.headRef == snapshot.headRef {
			if baseSHA != "" {
				pull.baseSHA = baseSHA
			}
			if headSHA != "" {
				pull.headSHA = headSHA
			}
			w.pulls[snapshot.key] = pull
		}
		w.mu.Unlock()
	}
}
