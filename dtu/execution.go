package dtu

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	pinnedActVersion   = "0.2.89"
	pinnedRunnerImage  = "catthehacker/ubuntu:act-22.04"
	pinnedRunnerDigest = "catthehacker/ubuntu@sha256:93b433d1c736e9c4361edf3bd4ea47434fa6323c4e70fdf34f826280584bab2d"
)

func (w *world) executeWorkflowRun(response http.ResponseWriter, request *http.Request) {
	var input ExecuteWorkflowInput
	if !decodeControl(response, request, &input) {
		return
	}
	w.mu.Lock()
	run, index, found := w.workflowRunByID(input.RunID)
	if !found {
		w.mu.Unlock()
		writeControlError(response, http.StatusNotFound, "unknown workflow run")
		return
	}
	if run.Status != "queued" {
		w.mu.Unlock()
		writeControlError(response, http.StatusConflict, "workflow run is not queued")
		return
	}
	if _, executing := w.activeRuns[run.ID]; executing {
		w.mu.Unlock()
		writeControlError(response, http.StatusConflict, "workflow run execution is already claimed")
		return
	}
	repository, repositoryFound := w.repositories[run.RepositoryID]
	workflow, workflowFound := w.workflows[run.RepositoryID]
	if !repositoryFound || !workflowFound {
		w.mu.Unlock()
		writeControlError(response, http.StatusConflict, "workflow run configuration is unavailable")
		return
	}
	w.activeRuns[run.ID] = activeRun{}
	w.mu.Unlock()
	claimed := true
	defer func() {
		if !claimed {
			return
		}
		w.mu.Lock()
		delete(w.activeRuns, run.ID)
		w.mu.Unlock()
	}()

	actPath, err := exec.LookPath("act")
	if err != nil {
		writeControlError(response, http.StatusServiceUnavailable, "act is not installed")
		return
	}
	version, err := exec.Command(actPath, "--version").Output()
	if err != nil || !strings.Contains(string(version), pinnedActVersion) {
		writeControlError(response, http.StatusServiceUnavailable, "act version does not match the pinned proof version")
		return
	}
	runnerArchitecture, err := verifyRunnerImage()
	if err != nil {
		writeControlError(response, http.StatusServiceUnavailable, err.Error())
		return
	}
	dockerConfigDir, err := w.prepareDockerConfig()
	if err != nil {
		writeControlError(response, http.StatusInternalServerError, "prepare isolated Docker configuration")
		return
	}

	checkoutDir := filepath.Join(w.dataDir, "workflow-runs", fmt.Sprintf("%d", run.ID))
	if err := os.RemoveAll(checkoutDir); err != nil {
		writeControlError(response, http.StatusInternalServerError, "prepare workflow checkout")
		return
	}
	if err := os.MkdirAll(filepath.Dir(checkoutDir), 0o755); err != nil {
		writeControlError(response, http.StatusInternalServerError, "prepare workflow directory")
		return
	}
	if err := runGit("", "clone", "--no-checkout", repository.gitDir, checkoutDir); err != nil {
		writeControlError(response, http.StatusInternalServerError, "clone workflow checkout")
		return
	}
	if err := runGit(checkoutDir, "checkout", "--detach", run.HeadSHA); err != nil {
		writeControlError(response, http.StatusInternalServerError, "checkout workflow SHA")
		return
	}
	if got := resolveCheckoutHEAD(checkoutDir); got != run.HeadSHA {
		writeControlError(response, http.StatusInternalServerError, "workflow checkout SHA mismatch")
		return
	}

	command := exec.Command(actPath,
		"push",
		"--workflows", workflow.path,
		"--platform", "ubuntu-latest="+pinnedRunnerImage,
		"--pull=false",
		"--bind",
		"--container-options", "--label=dtu.run_id="+strconv.FormatInt(run.ID, 10)+" --label=dtu.instance="+w.runnerInstanceLabel(),
		"--container-architecture", "linux/"+runnerArchitecture,
		"--rm",
	)
	command.Dir = checkoutDir
	command.Env = append(os.Environ(), "DOCKER_CONFIG="+dockerConfigDir)
	var logs bytes.Buffer
	fmt.Fprintf(&logs, "dtu: run_id=%d head_sha=%s runner=%s digest=%s\n", run.ID, run.HeadSHA, pinnedRunnerImage, pinnedRunnerDigest)
	command.Stdout = &logs
	command.Stderr = &logs

	w.mu.Lock()
	run = w.workflowRuns[index]
	if run.Status != "queued" {
		delete(w.activeRuns, run.ID)
		claimed = false
		w.mu.Unlock()
		writeControlError(response, http.StatusConflict, "workflow run is no longer queued")
		return
	}
	if run.CancellationRequested {
		delete(w.activeRuns, run.ID)
		claimed = false
		run.Status = "completed"
		run.Conclusion = "cancelled"
		w.workflowRuns[index] = run
		w.appendWorkflowEvent(repository, run, "completed")
		w.mutations++
		w.mu.Unlock()
		writeJSON(response, http.StatusOK, run)
		return
	}
	if err := command.Start(); err != nil {
		delete(w.activeRuns, run.ID)
		claimed = false
		w.mu.Unlock()
		writeControlError(response, http.StatusInternalServerError, "start workflow runner")
		return
	}
	run.Status = "in_progress"
	w.workflowRuns[index] = run
	w.appendWorkflowEvent(repository, run, "in_progress")
	w.activeRuns[run.ID] = activeRun{command: command}
	claimed = false
	w.mutations++
	w.mu.Unlock()
	waitErr := command.Wait()

	w.mu.Lock()
	run = w.workflowRuns[index]
	active := w.activeRuns[run.ID]
	delete(w.activeRuns, run.ID)
	run.Status = "completed"
	run.Logs = logs.String()
	switch {
	case active.cancellationSignalled && waitErr != nil:
		run.Conclusion = "cancelled"
	case waitErr == nil:
		run.Conclusion = "success"
	default:
		run.Conclusion = "failure"
	}
	w.workflowRuns[index] = run
	w.appendWorkflowEvent(repository, run, "completed")
	w.mutations++
	w.mu.Unlock()
	if active.cancellationSignalled {
		w.cleanupRunContainers(run.ID)
	}
	writeJSON(response, http.StatusOK, run)
}

func (w *world) prepareDockerConfig() (string, error) {
	dir := filepath.Join(w.dataDir, "docker-config")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}\n"), 0o600); err != nil {
		return "", err
	}
	return dir, nil
}

func (w *world) runnerInstanceLabel() string {
	digest := sha256.Sum256([]byte(w.dataDir))
	return fmt.Sprintf("%x", digest[:8])
}

func (w *world) cleanupRunContainers(runID int64) {
	output, err := exec.Command(
		"docker", "ps", "-aq",
		"--filter", "label=dtu.instance="+w.runnerInstanceLabel(),
		"--filter", "label=dtu.run_id="+strconv.FormatInt(runID, 10),
	).Output()
	if err != nil {
		return
	}
	for _, containerID := range strings.Fields(string(output)) {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
	}
}

func (w *world) workflowRunByID(runID int64) (WorkflowRun, int, bool) {
	for index, run := range w.workflowRuns {
		if run.ID == runID {
			return run, index, true
		}
	}
	return WorkflowRun{}, -1, false
}

func resolveCheckoutHEAD(dir string) string {
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func verifyRunnerImage() (string, error) {
	output, err := exec.Command(
		"docker", "image", "inspect", pinnedRunnerImage,
		"--format", "{{json .RepoDigests}}",
	).Output()
	if err != nil {
		return "", fmt.Errorf("pinned runner image is not installed")
	}
	var digests []string
	if err := json.Unmarshal(bytes.TrimSpace(output), &digests); err != nil {
		return "", fmt.Errorf("inspect pinned runner image digest")
	}
	digestMatches := false
	for _, digest := range digests {
		if digest == pinnedRunnerDigest {
			digestMatches = true
			break
		}
	}
	if !digestMatches {
		return "", fmt.Errorf("runner image does not match the pinned proof digest")
	}
	output, err = exec.Command(
		"docker", "image", "inspect", pinnedRunnerImage,
		"--format", "{{.Architecture}}",
	).Output()
	if err != nil {
		return "", fmt.Errorf("inspect pinned runner image architecture")
	}
	architecture := strings.TrimSpace(string(output))
	if architecture != "amd64" && architecture != "arm64" {
		return "", fmt.Errorf("runner image architecture is unsupported")
	}
	return architecture, nil
}
