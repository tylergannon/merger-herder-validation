package dtu_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tylergannon/merger-herder-validation/dtu"
)

func TestInstallationRepositoryBootstrapEvents(t *testing.T) {
	received := make(chan string, 2)
	receiver := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		received <- request.Header.Get("X-GitHub-Event") + ":" + string(body)
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(receiver.Close)
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	must(t, control.CreateApp(t.Context(), dtu.AppInput{
		ID: 1, PublicKeyPEM: publicKeyPEM(t, privateKey), WebhookURL: receiver.URL, WebhookSecret: "proof-secret",
	}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 10, AppID: 1, Active: true, Owner: "Acme", OwnerType: "Organization",
		Permissions: map[string]string{"actions": "write", "contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{
		ID: 100, Owner: "Acme", Name: "widget", InstallationID: 10,
	}))

	pending := state(t, control).PendingEvents
	if len(pending) != 2 || pending[0].Event != "installation" || pending[0].Action != "created" || pending[1].Event != "installation_repositories" || pending[1].Action != "added" {
		t.Fatalf("unexpected bootstrap events: %#v", pending)
	}
	var payload struct {
		Installation struct {
			ID      int64 `json:"id"`
			Account struct {
				Login string `json:"login"`
				Type  string `json:"type"`
			} `json:"account"`
		} `json:"installation"`
		RepositoriesAdded []struct {
			ID            int64  `json:"id"`
			FullName      string `json:"full_name"`
			DefaultBranch string `json:"default_branch"`
		} `json:"repositories_added"`
	}
	if err := json.Unmarshal(pending[1].Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Installation.ID != 10 || payload.Installation.Account.Login != "Acme" || payload.Installation.Account.Type != "Organization" || len(payload.RepositoriesAdded) != 1 || payload.RepositoriesAdded[0].ID != 100 || payload.RepositoriesAdded[0].FullName != "Acme/widget" || payload.RepositoriesAdded[0].DefaultBranch != "main" {
		t.Fatalf("unexpected repository event payload: %#v", payload)
	}
	for _, event := range pending {
		must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: event.GUID}))
	}
	if first, second := <-received, <-received; !strings.HasPrefix(first, "installation:") || !strings.HasPrefix(second, "installation_repositories:") {
		t.Fatalf("unexpected delivered events: %q %q", first, second)
	}
}
