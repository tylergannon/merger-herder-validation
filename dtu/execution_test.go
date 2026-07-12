package dtu_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/merger-herder-validation/dtu"
)

func TestActWorkflowExecutionAtExactSHA(t *testing.T) {
	requireActRuntime(t)
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	seedAppInstallationRepository(t, control, privateKey, 1, 10, 100, "Acme", "widget")
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))
	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	issued, _, err := githubClient(instance.GitHubURL, appJWT).Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture := createGitFixture(t)
	run(t, fixture, "git", "checkout", "feature")
	must(t, os.MkdirAll(filepath.Join(fixture, ".github/workflows"), 0o755))
	mustWrite(t, filepath.Join(fixture, "candidate.txt"), "release-candidate\n")
	mustWrite(t, filepath.Join(fixture, ".github/workflows/ci.yml"), `name: CI
on: push
jobs:
  prove:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5
      - name: Verify exact candidate
        run: test "$(cat candidate.txt)" = "release-candidate"
`)
	run(t, fixture, "git", "add", ".github/workflows/ci.yml", "candidate.txt")
	run(t, fixture, "git", "commit", "-m", "add exact candidate workflow")
	releaseSHA := output(t, fixture, "git", "rev-parse", "HEAD")
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "push", remote, "feature:refs/heads/R")
	created := state(t, control).WorkflowRuns[0]
	if created.HeadSHA != releaseSHA {
		t.Fatalf("run SHA = %s, want %s", created.HeadSHA, releaseSHA)
	}
	must(t, control.ExecuteWorkflowRun(t.Context(), dtu.ExecuteWorkflowInput{RunID: created.ID}))
	completed := workflowRun(t, control, created.ID)
	if completed.Status != "completed" || completed.Conclusion != "success" || completed.HeadSHA != releaseSHA || !strings.Contains(completed.Logs, "head_sha="+releaseSHA) || !strings.Contains(completed.Logs, "actions/checkout") || !strings.Contains(completed.Logs, "Verify exact candidate") {
		t.Fatalf("unexpected act-backed run: %#v", completed)
	}
}

func TestActWorkflowCancellationStopsSupervisor(t *testing.T) {
	requireActRuntime(t)
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	must(t, control.CreateApp(t.Context(), dtu.AppInput{ID: 1, PublicKeyPEM: publicKeyPEM(t, privateKey)}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 10, AppID: 1, Active: true,
		Permissions: map[string]string{"actions": "write", "contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{
		ID: 100, Owner: "Acme", Name: "widget", InstallationID: 10,
	}))
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))
	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	issued, _, err := githubClient(instance.GitHubURL, appJWT).Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture := createGitFixture(t)
	run(t, fixture, "git", "checkout", "feature")
	must(t, os.MkdirAll(filepath.Join(fixture, ".github/workflows"), 0o755))
	mustWrite(t, filepath.Join(fixture, ".github/workflows/ci.yml"), `name: CI
on: push
jobs:
  prove:
    runs-on: ubuntu-latest
    steps:
      - name: Wait for cancellation
        shell: sh
        run: sleep 30
`)
	run(t, fixture, "git", "add", ".github/workflows/ci.yml")
	run(t, fixture, "git", "commit", "-m", "add cancellable workflow")
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "push", remote, "feature:refs/heads/R")
	created := state(t, control).WorkflowRuns[0]

	executionContext, stopExecution := context.WithTimeout(t.Context(), 20*time.Second)
	defer stopExecution()
	executed := make(chan error, 1)
	go func() {
		executed <- control.ExecuteWorkflowRun(executionContext, dtu.ExecuteWorkflowInput{RunID: created.ID})
	}()
	waitForWorkflowStatus(t, control, created.ID, "in_progress")
	container := waitForRunContainer(t, created.ID)
	err = control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: created.ID, Status: "completed", Conclusion: "success"})
	if err == nil || !strings.Contains(err.Error(), "workflow run execution is claimed") {
		t.Fatalf("scripted transition during execution error = %v", err)
	}
	err = control.ExecuteWorkflowRun(t.Context(), dtu.ExecuteWorkflowInput{RunID: created.ID})
	if err == nil || !strings.Contains(err.Error(), "workflow run is not queued") {
		t.Fatalf("second executor claim error = %v", err)
	}
	client := githubClient(instance.GitHubURL, issued.GetToken())
	response, err := client.Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", created.ID)
	assertAcceptedCancellation(t, response, err)
	select {
	case err := <-executed:
		must(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("act supervisor did not stop after cancellation")
	}
	completed := workflowRun(t, control, created.ID)
	if completed.Status != "completed" || completed.Conclusion != "cancelled" || !completed.CancellationRequested || !strings.Contains(completed.Logs, "head_sha="+created.HeadSHA) {
		t.Fatalf("unexpected cancelled act-backed run: %#v", completed)
	}
	assertNoRunContainers(t, container)
}

func TestActWorkflowShutdownStopsSupervisor(t *testing.T) {
	requireActRuntime(t)
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	seedAppInstallationRepository(t, control, privateKey, 1, 10, 100, "Acme", "widget")
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))
	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	issued, _, err := githubClient(instance.GitHubURL, appJWT).Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture := createGitFixture(t)
	run(t, fixture, "git", "checkout", "feature")
	must(t, os.MkdirAll(filepath.Join(fixture, ".github/workflows"), 0o755))
	mustWrite(t, filepath.Join(fixture, ".github/workflows/ci.yml"), `name: CI
on: push
jobs:
  prove:
    runs-on: ubuntu-latest
    steps:
      - name: Wait for shutdown
        run: sleep 30
`)
	run(t, fixture, "git", "add", ".github/workflows/ci.yml")
	run(t, fixture, "git", "commit", "-m", "add shutdown workflow")
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "push", remote, "feature:refs/heads/R")
	created := state(t, control).WorkflowRuns[0]

	executed := make(chan error, 1)
	go func() {
		executed <- control.ExecuteWorkflowRun(t.Context(), dtu.ExecuteWorkflowInput{RunID: created.ID})
	}()
	waitForWorkflowStatus(t, control, created.ID, "in_progress")
	container := waitForRunContainer(t, created.ID)
	shutdownContext, stopShutdown := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopShutdown()
	must(t, instance.Close(shutdownContext))
	select {
	case <-executed:
	case <-time.After(5 * time.Second):
		t.Fatal("workflow execution request remained blocked after DTU shutdown")
	}
	assertNoRunContainers(t, container)
}

func waitForWorkflowStatus(t *testing.T, control dtu.ControlClient, runID int64, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if workflowRun(t, control, runID).Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workflow run %d did not reach %s", runID, want)
}

type runContainer struct {
	runID    int64
	instance string
}

func waitForRunContainer(t *testing.T, runID int64) runContainer {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, containerID := range runContainerIDs(t, runContainer{runID: runID}) {
			output, err := exec.Command(
				"docker", "inspect", containerID,
				"--format", `{{index .Config.Labels "dtu.instance"}}`,
			).Output()
			if err == nil && strings.TrimSpace(string(output)) != "" {
				return runContainer{runID: runID, instance: strings.TrimSpace(string(output))}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workflow run %d did not start a labeled container", runID)
	return runContainer{}
}

func assertNoRunContainers(t *testing.T, container runContainer) {
	t.Helper()
	if containers := runContainerIDs(t, container); len(containers) > 0 {
		t.Fatalf("workflow run %d leaked containers: %v", container.runID, containers)
	}
}

func runContainerIDs(t *testing.T, container runContainer) []string {
	t.Helper()
	arguments := []string{"ps", "-aq", "--filter", "label=dtu.run_id=" + strconv.FormatInt(container.runID, 10)}
	if container.instance != "" {
		arguments = append(arguments, "--filter", "label=dtu.instance="+container.instance)
	}
	output, err := exec.Command("docker", arguments...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(output))
}

func requireActRuntime(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("act"); err != nil {
		actRuntimeUnavailable(t, "act is not installed")
	}
	output, err := exec.Command(
		"docker", "image", "inspect", "catthehacker/ubuntu:act-22.04",
		"--format", "{{json .RepoDigests}}",
	).Output()
	if err != nil || !strings.Contains(string(output), "catthehacker/ubuntu@sha256:93b433d1c736e9c4361edf3bd4ea47434fa6323c4e70fdf34f826280584bab2d") {
		actRuntimeUnavailable(t, "pinned runner image is not installed")
	}
}

func actRuntimeUnavailable(t *testing.T, message string) {
	t.Helper()
	if os.Getenv("DTU_REQUIRE_ACT") == "1" {
		t.Fatal(message)
	}
	t.Skip(message)
}
