package dtu_test

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/tylergannon/merger-herder-validation/dtu"
)

func TestGITRefTransitionsAndEventCreation(t *testing.T) {
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	seedAppInstallationRepository(t, control, privateKey, 1, 10, 100, "Acme", "widget")
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100,
		ID:           77,
		Name:         "CI",
		Path:         ".github/workflows/ci.yml",
		ReleaseRef:   "R",
	}))

	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	appClient := githubClient(instance.GitHubURL, appJWT)
	issued, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	fixture := createGitFixture(t)
	run(t, fixture, "git", "remote", "add", "origin", remote)
	run(t, fixture, "git", "push", "origin", "main", "feature")
	baseSHA := output(t, fixture, "git", "rev-parse", "main")
	featureSHA := output(t, fixture, "git", "rev-parse", "feature")

	proveExactObjectFetch(t, remote, baseSHA, featureSHA)
	proveRejectedPushHasNoEvent(t, control, appClient, instance.GitURL, fixture)

	before := state(t, control)
	run(t, fixture, "git", "push", "origin", "feature:refs/heads/R")
	afterCreate := state(t, control)
	assertReleaseTransition(t, before, afterCreate, zeroSHA(), featureSHA, false)

	run(t, fixture, "git", "checkout", "feature")
	mustWrite(t, filepath.Join(fixture, "release.txt"), "fast-forward\n")
	run(t, fixture, "git", "add", "release.txt")
	run(t, fixture, "git", "commit", "-m", "advance release")
	advancedSHA := output(t, fixture, "git", "rev-parse", "feature")
	before = afterCreate
	run(t, fixture, "git", "push", "origin", "feature:refs/heads/R")
	afterAdvance := state(t, control)
	assertReleaseTransition(t, before, afterAdvance, featureSHA, advancedSHA, false)

	run(t, fixture, "git", "checkout", "-b", "repair", "main")
	mustWrite(t, filepath.Join(fixture, "repair.txt"), "repair\n")
	run(t, fixture, "git", "add", "repair.txt")
	run(t, fixture, "git", "commit", "-m", "rebuild release")
	repairSHA := output(t, fixture, "git", "rev-parse", "repair")
	before = afterAdvance
	run(t, fixture, "git", "push", "--force-with-lease=refs/heads/R:"+advancedSHA, "origin", "repair:refs/heads/R")
	afterRepair := state(t, control)
	assertReleaseTransition(t, before, afterRepair, advancedSHA, repairSHA, true)

	before = afterRepair
	runFails(t, fixture, "git", "push", "--force-with-lease=refs/heads/R:"+advancedSHA, "origin", "feature:refs/heads/R")
	afterStaleLease := state(t, control)
	assertNoGitHubStateChange(t, before, afterStaleLease)
	if got := remoteRef(t, remote, "R"); got != repairSHA {
		t.Fatalf("stale lease moved R to %s, want %s", got, repairSHA)
	}

	before = afterStaleLease
	run(t, fixture, "git", "push", "origin", "repair:refs/heads/main")
	afterLanding := state(t, control)
	assertPushOnlyTransition(t, before, afterLanding, "main", baseSHA, repairSHA, false)

	before = afterLanding
	runFails(t, fixture, "git", "push", "origin", baseSHA+":refs/heads/main")
	afterNonFastForward := state(t, control)
	assertNoGitHubStateChange(t, before, afterNonFastForward)
	if got := remoteRef(t, remote, "main"); got != repairSHA {
		t.Fatalf("non-fast-forward moved main to %s, want %s", got, repairSHA)
	}

	before = afterNonFastForward
	run(t, fixture, "git", "push", "origin", ":refs/heads/feature")
	afterDelete := state(t, control)
	assertPushOnlyTransition(t, before, afterDelete, "feature", featureSHA, zeroSHA(), false)
	proveConcurrentPushObservation(t, control, fixture, remote, baseSHA)
}

func proveExactObjectFetch(t *testing.T, remote string, shas ...string) {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	for _, sha := range shas {
		run(t, dir, "git", "fetch", remote, sha)
		if fetched := output(t, dir, "git", "rev-parse", "FETCH_HEAD"); fetched != sha {
			t.Fatalf("fetched SHA = %s, want %s", fetched, sha)
		}
	}
	runFails(t, dir, "git", "fetch", remote, strings.Repeat("1", 40))
	unauthorized, err := url.Parse(remote)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized.User = url.UserPassword("x-access-token", "invalid")
	runFails(t, dir, "git", "fetch", unauthorized.String(), shas[0])
}

func proveRejectedPushHasNoEvent(t *testing.T, control dtu.ControlClient, appClient *github.Client, baseURL url.URL, fixture string) {
	t.Helper()
	readOnly, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		RepositoryIDs: []int64{100},
		Permissions:   &github.InstallationPermissions{Contents: new("read")},
	})
	if err != nil {
		t.Fatal(err)
	}
	before := state(t, control)
	runFails(t, fixture, "git", "push", authenticatedGitURL(baseURL, "Acme", "widget", readOnly.GetToken()), "main:refs/heads/rejected")
	after := state(t, control)
	assertNoGitHubStateChange(t, before, after)
}

func assertReleaseTransition(t *testing.T, before, after dtu.StateSnapshot, beforeSHA, afterSHA string, forced bool) {
	t.Helper()
	if len(after.PendingEvents) != len(before.PendingEvents)+2 {
		t.Fatalf("pending events = %d, want %d", len(after.PendingEvents), len(before.PendingEvents)+2)
	}
	if len(after.WorkflowRuns) != len(before.WorkflowRuns)+1 {
		t.Fatalf("workflow runs = %d, want %d", len(after.WorkflowRuns), len(before.WorkflowRuns)+1)
	}
	assertPushEvent(t, after.PendingEvents[len(before.PendingEvents)], "R", beforeSHA, afterSHA, forced)
	workflowEvent := after.PendingEvents[len(before.PendingEvents)+1]
	run := after.WorkflowRuns[len(before.WorkflowRuns)]
	if workflowEvent.Event != "workflow_run" || workflowEvent.Action != "requested" || run.Status != "queued" || run.Conclusion != "" || run.Attempt != 1 || run.WorkflowID != 77 || run.HeadBranch != "R" || run.HeadSHA != afterSHA || run.Event != "push" {
		t.Fatalf("unexpected workflow creation: event=%#v run=%#v", workflowEvent, run)
	}
	var payload struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			ID         int64  `json:"id"`
			RunAttempt int    `json:"run_attempt"`
			HeadSHA    string `json:"head_sha"`
			Status     string `json:"status"`
			Conclusion any    `json:"conclusion"`
		} `json:"workflow_run"`
	}
	if err := json.Unmarshal(workflowEvent.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Action != "requested" || payload.WorkflowRun.ID != run.ID || payload.WorkflowRun.RunAttempt != 1 || payload.WorkflowRun.HeadSHA != afterSHA || payload.WorkflowRun.Status != "queued" || payload.WorkflowRun.Conclusion != nil {
		t.Fatalf("unexpected workflow event body: %#v", payload)
	}
}

func assertPushOnlyTransition(t *testing.T, before, after dtu.StateSnapshot, ref, beforeSHA, afterSHA string, forced bool) {
	t.Helper()
	if len(after.PendingEvents) != len(before.PendingEvents)+1 || len(after.WorkflowRuns) != len(before.WorkflowRuns) {
		t.Fatalf("unexpected push-only delta: before=%#v after=%#v", before, after)
	}
	assertPushEvent(t, after.PendingEvents[len(before.PendingEvents)], ref, beforeSHA, afterSHA, forced)
}

func assertPushEvent(t *testing.T, event dtu.PendingEvent, ref, beforeSHA, afterSHA string, forced bool) {
	t.Helper()
	if event.GUID == "" || event.Event != "push" || event.RepositoryID != 100 || event.CreatedAt != proofTime {
		t.Fatalf("unexpected pending push event: %#v", event)
	}
	var payload struct {
		Ref     string `json:"ref"`
		Before  string `json:"before"`
		After   string `json:"after"`
		Created bool   `json:"created"`
		Deleted bool   `json:"deleted"`
		Forced  bool   `json:"forced"`
		Sender  struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(event.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Ref != "refs/heads/"+ref || payload.Before != beforeSHA || payload.After != afterSHA || payload.Created != (beforeSHA == zeroSHA()) || payload.Deleted != (afterSHA == zeroSHA()) || payload.Forced != forced || payload.Sender.ID != 1 || payload.Sender.Login != "dtu-app-1[bot]" || payload.Sender.Type != "Bot" {
		t.Fatalf("unexpected push body: %#v", payload)
	}
}

func assertNoGitHubStateChange(t *testing.T, before, after dtu.StateSnapshot) {
	t.Helper()
	if len(after.PendingEvents) != len(before.PendingEvents) || len(after.WorkflowRuns) != len(before.WorkflowRuns) || len(after.ObservationErrors) != len(before.ObservationErrors) || after.Mutations != before.Mutations {
		t.Fatalf("rejected push changed GitHub state: before=%#v after=%#v", before, after)
	}
}

func proveConcurrentPushObservation(t *testing.T, control dtu.ControlClient, fixture, remote, sha string) {
	t.Helper()
	run(t, fixture, "git", "checkout", "-b", "race-one", "main")
	mustWrite(t, filepath.Join(fixture, "race-one.txt"), "one\n")
	run(t, fixture, "git", "add", "race-one.txt")
	run(t, fixture, "git", "commit", "-m", "race one")
	oneSHA := output(t, fixture, "git", "rev-parse", "race-one")
	run(t, fixture, "git", "checkout", "-b", "race-two", "main")
	mustWrite(t, filepath.Join(fixture, "race-two.txt"), "two\n")
	run(t, fixture, "git", "add", "race-two.txt")
	run(t, fixture, "git", "commit", "-m", "race two")
	twoSHA := output(t, fixture, "git", "rev-parse", "race-two")
	run(t, fixture, "git", "push", remote, "main:refs/heads/race")
	before := state(t, control)
	type result struct {
		branch string
		output string
		err    error
	}
	results := make(chan result, 2)
	for _, branch := range []string{"race-one", "race-two"} {
		go func() {
			command := exec.Command("git", "push", "--force-with-lease=refs/heads/race:"+sha, remote, branch+":refs/heads/race")
			command.Dir = fixture
			command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
			output, err := command.CombinedOutput()
			results <- result{branch: branch, output: string(output), err: err}
		}()
	}
	successes := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent exact-lease successes = %d, want 1", successes)
	}
	after := state(t, control)
	if len(after.PendingEvents) != len(before.PendingEvents)+1 || len(after.WorkflowRuns) != len(before.WorkflowRuns) || len(after.ObservationErrors) != 0 {
		t.Fatalf("unexpected concurrent push state: before=%#v after=%#v", before, after)
	}
	winner := remoteRef(t, remote, "race")
	if winner != oneSHA && winner != twoSHA {
		t.Fatalf("race ref = %s, want %s or %s", winner, oneSHA, twoSHA)
	}
	assertPushEvent(t, after.PendingEvents[len(before.PendingEvents)], "race", sha, winner, false)
}

func remoteRef(t *testing.T, remote, ref string) string {
	t.Helper()
	line := output(t, "", "git", "ls-remote", remote, "refs/heads/"+ref)
	fields := strings.Fields(line)
	if len(fields) != 2 {
		t.Fatalf("unexpected ls-remote output: %q", line)
	}
	return fields[0]
}

func zeroSHA() string {
	return strings.Repeat("0", 40)
}
