package dtu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type ControlClient struct {
	BaseURL    url.URL
	httpClient *http.Client
}

func NewControlClient(baseURL url.URL) ControlClient {
	return ControlClient{BaseURL: baseURL, httpClient: new(http.Client)}
}

func (c ControlClient) CreateApp(ctx context.Context, input AppInput) error {
	return c.post(ctx, "apps", input)
}

func (c ControlClient) CreateInstallation(ctx context.Context, input InstallationInput) error {
	return c.post(ctx, "installations", input)
}

func (c ControlClient) CreateRepository(ctx context.Context, input RepositoryInput) error {
	return c.post(ctx, "repositories", input)
}

func (c ControlClient) CreatePullRequest(ctx context.Context, input PullRequestInput) error {
	return c.post(ctx, "pulls", input)
}

func (c ControlClient) ChangePullRequestState(ctx context.Context, input PullRequestStateInput) error {
	return c.post(ctx, "pulls/state", input)
}

func (c ControlClient) AdvanceTime(ctx context.Context, input AdvanceTimeInput) error {
	return c.post(ctx, "clock/advance", input)
}

func (c ControlClient) ConfigureWorkflow(ctx context.Context, input WorkflowInput) error {
	return c.post(ctx, "workflows", input)
}

func (c ControlClient) State(ctx context.Context) (StateSnapshot, error) {
	endpoint := c.BaseURL.ResolveReference(&url.URL{Path: "state"})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return StateSnapshot{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return StateSnapshot{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return StateSnapshot{}, fmt.Errorf("control state returned %s", response.Status)
	}
	var result StateSnapshot
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return StateSnapshot{}, err
	}
	return result, nil
}

func (c ControlClient) post(ctx context.Context, path string, input any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	endpoint := c.BaseURL.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		return fmt.Errorf("control %s returned %s: %s", path, response.Status, failure.Message)
	}
	return nil
}
