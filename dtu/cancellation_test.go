package dtu_test

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/tylergannon/merger-herder-validation/dtu"
)

func TestWorkflowCancellationRaces(t *testing.T) {
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	must(t, control.CreateApp(t.Context(), dtu.AppInput{ID: 1, PublicKeyPEM: publicKeyPEM(t, privateKey)}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 10, AppID: 1, Active: true,
		Permissions: map[string]string{"actions": "write", "contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{ID: 100, Owner: "Acme", Name: "widget", InstallationID: 10}))
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))
	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	appClient := githubClient(instance.GitHubURL, appJWT)
	issued, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := githubClient(instance.GitHubURL, issued.GetToken())
	fixture := createGitFixture(t)
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "push", remote, "feature:refs/heads/R")
	firstRun := state(t, control).WorkflowRuns[0]

	response, err := client.Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", firstRun.ID)
	assertAcceptedCancellation(t, response, err)
	queuedCancelled := workflowRun(t, control, firstRun.ID)
	if queuedCancelled.Status != "queued" || !queuedCancelled.CancellationRequested {
		t.Fatalf("cancel acceptance changed terminal state: %#v", queuedCancelled)
	}
	// Acceptance does not prevent a racing success from becoming authoritative.
	must(t, control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: firstRun.ID, Status: "completed", Conclusion: "success"}))
	response, err = client.Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", firstRun.ID)
	assertGitHubError(t, err, response, http.StatusConflict)

	run(t, fixture, "git", "checkout", "feature")
	mustWrite(t, filepath.Join(fixture, "second.txt"), "second\n")
	run(t, fixture, "git", "add", "second.txt")
	run(t, fixture, "git", "commit", "-m", "second run")
	run(t, fixture, "git", "push", remote, "feature:refs/heads/R")
	runs := state(t, control).WorkflowRuns
	secondRun := runs[len(runs)-1]
	must(t, control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: secondRun.ID, Status: "in_progress"}))
	response, err = client.Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", secondRun.ID)
	assertAcceptedCancellation(t, response, err)
	must(t, control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: secondRun.ID, Status: "completed", Conclusion: "cancelled"}))
	completedCancelled := workflowRun(t, control, secondRun.ID)
	if completedCancelled.Status != "completed" || completedCancelled.Conclusion != "cancelled" || !completedCancelled.CancellationRequested {
		t.Fatalf("unexpected cancelled terminal run: %#v", completedCancelled)
	}

	response, err = client.Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", 999999)
	assertGitHubError(t, err, response, http.StatusNotFound)
	response, err = githubClient(instance.GitHubURL, "invalid-token").Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", secondRun.ID)
	assertGitHubError(t, err, response, http.StatusUnauthorized)
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{ID: 101, Owner: "Acme", Name: "other", InstallationID: 10}))
	otherRepo, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		RepositoryIDs: []int64{101}, Permissions: &github.InstallationPermissions{Actions: new("write")},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err = githubClient(instance.GitHubURL, otherRepo.GetToken()).Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", secondRun.ID)
	assertGitHubError(t, err, response, http.StatusNotFound)
	noActions, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		RepositoryIDs: []int64{100}, Permissions: &github.InstallationPermissions{Contents: new("write")},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err = githubClient(instance.GitHubURL, noActions.GetToken()).Actions.CancelWorkflowRunByID(t.Context(), "Acme", "widget", secondRun.ID)
	assertGitHubError(t, err, response, http.StatusNotFound)
}

func assertAcceptedCancellation(t *testing.T, response *github.Response, err error) {
	t.Helper()
	var accepted *github.AcceptedError
	if !errors.As(err, &accepted) || response == nil || response.StatusCode != http.StatusAccepted {
		t.Fatalf("cancellation response=%#v error=%v, want AcceptedError/202", response, err)
	}
}

func workflowRun(t *testing.T, control dtu.ControlClient, runID int64) dtu.WorkflowRun {
	t.Helper()
	for _, run := range state(t, control).WorkflowRuns {
		if run.ID == runID {
			return run
		}
	}
	t.Fatalf("missing workflow run %d", runID)
	return dtu.WorkflowRun{}
}
