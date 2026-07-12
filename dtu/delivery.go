package dtu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

func (w *world) deliverEvent(response http.ResponseWriter, request *http.Request) {
	var input DeliveryInput
	if !decodeControl(response, request, &input) {
		return
	}
	w.mu.RLock()
	event, found := w.findPendingEvent(input.GUID)
	repository := w.repositories[event.RepositoryID]
	installation := w.installs[repository.installationID]
	configuredApp := w.apps[installation.appID]
	now := w.now
	w.mu.RUnlock()
	if !found {
		writeControlError(response, http.StatusNotFound, "unknown event")
		return
	}
	if configuredApp.webhookURL == "" || configuredApp.webhookSecret == "" {
		writeControlError(response, http.StatusConflict, "webhook destination is not configured")
		return
	}

	secret := configuredApp.webhookSecret
	if input.InvalidSignature {
		secret += "-invalid"
	}
	signature := webhookSignature([]byte(secret), event.Body)
	outbound, err := http.NewRequestWithContext(request.Context(), http.MethodPost, configuredApp.webhookURL, bytes.NewReader(event.Body))
	if err != nil {
		writeControlError(response, http.StatusBadRequest, "invalid webhook destination")
		return
	}
	outbound.Header.Set("Content-Type", "application/json")
	outbound.Header.Set("X-GitHub-Delivery", event.GUID)
	outbound.Header.Set("X-GitHub-Event", event.Event)
	outbound.Header.Set("X-Hub-Signature-256", signature)
	client := http.Client{Timeout: 10 * time.Second}
	deliveryResponse, deliveryErr := client.Do(outbound)
	attempt := DeliveryAttempt{
		GUID:             event.GUID,
		Destination:      configuredApp.webhookURL,
		InvalidSignature: input.InvalidSignature,
		AttemptedAt:      now,
	}
	if deliveryErr != nil {
		attempt.Error = deliveryErr.Error()
	} else {
		attempt.StatusCode = deliveryResponse.StatusCode
		deliveryResponse.Body.Close()
	}
	w.mu.Lock()
	w.deliveryAttempts = append(w.deliveryAttempts, attempt)
	w.mutations++
	w.mu.Unlock()
	if deliveryErr != nil {
		writeControlError(response, http.StatusBadGateway, "webhook delivery failed")
		return
	}
	if attempt.StatusCode < 200 || attempt.StatusCode >= 300 {
		writeControlError(response, http.StatusBadGateway, "webhook receiver rejected delivery")
		return
	}
	writeJSON(response, http.StatusOK, attempt)
}

func (w *world) duplicateEvent(response http.ResponseWriter, request *http.Request) {
	var input DuplicateEventInput
	if !decodeControl(response, request, &input) {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	source, found := w.findPendingEvent(input.GUID)
	if !found {
		writeControlError(response, http.StatusNotFound, "unknown event")
		return
	}
	w.nextEventID++
	duplicate := source
	duplicate.GUID = fmt.Sprintf("dtu-%012d", w.nextEventID)
	duplicate.CreatedAt = w.now
	duplicate.Body = append([]byte(nil), source.Body...)
	w.pendingEvents = append(w.pendingEvents, duplicate)
	w.mutations++
	writeJSON(response, http.StatusCreated, duplicate)
}

func (w *world) transitionWorkflowRun(response http.ResponseWriter, request *http.Request) {
	var input WorkflowTransitionInput
	if !decodeControl(response, request, &input) {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	index := -1
	for candidate := range w.workflowRuns {
		if w.workflowRuns[candidate].ID == input.RunID {
			index = candidate
			break
		}
	}
	if index < 0 {
		writeControlError(response, http.StatusNotFound, "unknown workflow run")
		return
	}
	if _, claimed := w.activeRuns[input.RunID]; claimed {
		writeControlError(response, http.StatusConflict, "workflow run execution is claimed")
		return
	}
	run := w.workflowRuns[index]
	action := ""
	switch {
	case run.Status == "queued" && input.Status == "in_progress" && input.Conclusion == "":
		action = "in_progress"
	case (run.Status == "queued" || run.Status == "in_progress") && input.Status == "completed" && input.Conclusion != "":
		action = "completed"
	default:
		writeControlError(response, http.StatusConflict, "invalid workflow transition")
		return
	}
	run.Status = input.Status
	run.Conclusion = input.Conclusion
	w.workflowRuns[index] = run
	repository := w.repositories[run.RepositoryID]
	w.appendWorkflowEvent(repository, run, action)
	w.mutations++
	writeJSON(response, http.StatusOK, run)
}

func (w *world) findPendingEvent(guid string) (PendingEvent, bool) {
	for _, event := range w.pendingEvents {
		if event.GUID == guid {
			return event, true
		}
	}
	return PendingEvent{}, false
}

func webhookSignature(secret, body []byte) string {
	digest := hmac.New(sha256.New, secret)
	_, _ = digest.Write(body)
	return "sha256=" + hex.EncodeToString(digest.Sum(nil))
}
