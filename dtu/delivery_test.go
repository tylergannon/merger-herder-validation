package dtu_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tylergannon/merger-herder-validation/dtu"
)

func TestWebhookDeliveryAndWorkflowLifecycle(t *testing.T) {
	const secret = "phase-two-secret"
	deliveries := make(chan capturedDelivery, 8)
	receiver := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		captured := capturedDelivery{
			GUID:      request.Header.Get("X-GitHub-Delivery"),
			Event:     request.Header.Get("X-GitHub-Event"),
			Signature: request.Header.Get("X-Hub-Signature-256"),
			Body:      body,
		}
		deliveries <- captured
		if !validSignature(secret, body, captured.Signature) {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		response.WriteHeader(http.StatusAccepted)
	}))
	defer receiver.Close()

	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	must(t, control.CreateApp(t.Context(), dtu.AppInput{
		ID: 1, PublicKeyPEM: publicKeyPEM(t, privateKey),
		WebhookURL: receiver.URL, WebhookSecret: secret,
	}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 10, AppID: 1, Active: true,
		Permissions: map[string]string{"contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{ID: 100, Owner: "Acme", Name: "widget", InstallationID: 10}))
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))

	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	issued, _, err := githubClient(instance.GitHubURL, appJWT).Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture := createGitFixture(t)
	featureSHA := output(t, fixture, "git", "rev-parse", "feature")
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "push", remote, "main", "feature", "feature:refs/heads/R")

	created := state(t, control)
	push := findEvent(t, created.PendingEvents, "push", "")
	assertDeliveredPushIdentity(t, push, featureSHA)
	requested := findEvent(t, created.PendingEvents, "workflow_run", "requested")
	if len(created.DeliveryAttempts) != 0 {
		t.Fatal("events delivered without control request")
	}
	runID := created.WorkflowRuns[0].ID
	assertWorkflowBody(t, requested, runID, "queued", nil)
	must(t, control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: runID, Status: "in_progress"}))
	must(t, control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: runID, Status: "completed", Conclusion: "success"}))
	transitioned := state(t, control)
	inProgress := findEvent(t, transitioned.PendingEvents, "workflow_run", "in_progress")
	completed := findEvent(t, transitioned.PendingEvents, "workflow_run", "completed")
	assertWorkflowBody(t, inProgress, runID, "in_progress", nil)
	assertWorkflowBody(t, completed, runID, "completed", "success")
	must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: push.GUID}))
	assertDelivery(t, <-deliveries, push, secret, true)

	// Delivery order is caller-controlled: completed is sent before in-progress.
	must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: completed.GUID}))
	first := <-deliveries
	assertDelivery(t, first, completed, secret, true)
	if err := control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: inProgress.GUID, InvalidSignature: true}); err == nil {
		t.Fatal("invalid-signature delivery did not report receiver rejection")
	}
	invalid := <-deliveries
	assertDelivery(t, invalid, inProgress, secret, false)

	// Redelivery preserves both GUID and exact bytes.
	must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: completed.GUID}))
	redelivery := <-deliveries
	if redelivery.GUID != first.GUID || string(redelivery.Body) != string(first.Body) {
		t.Fatalf("redelivery changed identity or body: first=%#v second=%#v", first, redelivery)
	}

	// A semantic duplicate gets a new GUID while preserving the immutable body.
	must(t, control.DuplicateEvent(t.Context(), dtu.DuplicateEventInput{GUID: completed.GUID}))
	duplicatedState := state(t, control)
	duplicate := duplicatedState.PendingEvents[len(duplicatedState.PendingEvents)-1]
	if duplicate.GUID == completed.GUID || string(duplicate.Body) != string(completed.Body) {
		t.Fatalf("invalid semantic duplicate: source=%#v duplicate=%#v", completed, duplicate)
	}
	must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: duplicate.GUID}))
	assertDelivery(t, <-deliveries, duplicate, secret, true)

	// Requested remains withheld, and a completed run cannot transition again.
	if requested.GUID == "" {
		t.Fatal("missing requested event")
	}
	if err := control.TransitionWorkflowRun(t.Context(), dtu.WorkflowTransitionInput{RunID: runID, Status: "completed", Conclusion: "failure"}); err == nil {
		t.Fatal("completed workflow accepted another transition")
	}
	final := state(t, control)
	if len(final.DeliveryAttempts) != 5 {
		t.Fatalf("delivery attempts = %d, want 5", len(final.DeliveryAttempts))
	}
	if final.DeliveryAttempts[0].GUID != push.GUID || final.DeliveryAttempts[0].StatusCode != 202 || final.DeliveryAttempts[1].StatusCode != 202 || final.DeliveryAttempts[2].StatusCode != 401 || final.DeliveryAttempts[3].GUID != completed.GUID || final.DeliveryAttempts[4].GUID != duplicate.GUID {
		t.Fatalf("unexpected attempts: %#v", final.DeliveryAttempts)
	}
}

func assertDeliveredPushIdentity(t *testing.T, event dtu.PendingEvent, afterSHA string) {
	t.Helper()
	var body struct {
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	if err := json.Unmarshal(event.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Ref != "refs/heads/R" || body.After != afterSHA {
		t.Fatalf("push identity = %#v, want R at %s", body, afterSHA)
	}
}

type capturedDelivery struct {
	GUID      string
	Event     string
	Signature string
	Body      []byte
}

func findEvent(t *testing.T, events []dtu.PendingEvent, event, action string) dtu.PendingEvent {
	t.Helper()
	for _, candidate := range events {
		if candidate.Event == event && candidate.Action == action {
			return candidate
		}
	}
	t.Fatalf("missing %s/%s event", event, action)
	return dtu.PendingEvent{}
}

func assertDelivery(t *testing.T, captured capturedDelivery, expected dtu.PendingEvent, secret string, valid bool) {
	t.Helper()
	if captured.GUID != expected.GUID || captured.Event != expected.Event || string(captured.Body) != string(expected.Body) || validSignature(secret, captured.Body, captured.Signature) != valid {
		t.Fatalf("unexpected delivery: captured=%#v expected=%#v valid=%v", captured, expected, valid)
	}
}

func validSignature(secret string, body []byte, signature string) bool {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write(body)
	want := "sha256=" + hex.EncodeToString(digest.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

func assertWorkflowBody(t *testing.T, event dtu.PendingEvent, runID int64, status string, wireConclusion any) {
	t.Helper()
	var body struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			ID         int64  `json:"id"`
			RunAttempt int    `json:"run_attempt"`
			Status     string `json:"status"`
			Conclusion any    `json:"conclusion"`
		} `json:"workflow_run"`
	}
	if err := json.Unmarshal(event.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Action != event.Action || body.WorkflowRun.ID != runID || body.WorkflowRun.RunAttempt != 1 || body.WorkflowRun.Status != status || body.WorkflowRun.Conclusion != wireConclusion {
		t.Fatalf("unexpected workflow body: %#v", body)
	}
}
