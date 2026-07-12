package dtu

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"time"
)

const tokenLifetime = time.Hour

type Config struct {
	PublicAddress  string
	ControlAddress string
	DataDir        string
	InitialTime    time.Time
}

type Instance struct {
	GitHubURL  url.URL
	GitURL     url.URL
	ControlURL url.URL
	runtime    *instanceRuntime
}

type instanceRuntime struct {
	publicServer  http.Server
	controlServer http.Server
	world         *world
	done          chan error
	removeDataDir bool
	dataDir       string
	closeOnce     sync.Once
	closeErr      error
}

func (i Instance) Close(ctx context.Context) error {
	return i.runtime.close(ctx)
}

type world struct {
	mu                sync.RWMutex
	now               time.Time
	dataDir           string
	gitBackend        string
	apps              map[int64]app
	installs          map[int64]installation
	repositories      map[int64]repository
	repoNames         map[string]int64
	pulls             map[pullKey]pullRequest
	tokens            map[string]installationToken
	workflows         map[int64]workflowConfig
	receiveLocks      map[int64]*sync.Mutex
	workflowRuns      []WorkflowRun
	pendingEvents     []PendingEvent
	observationErrors []ObservationError
	deliveryAttempts  []DeliveryAttempt
	activeRuns        map[int64]activeRun
	nextRunID         int64
	nextEventID       uint64
	unsupported       []UnsupportedRequest
	mutations         uint64
}

type app struct {
	id            int64
	publicKey     *rsa.PublicKey
	webhookURL    string
	webhookSecret string
}

type installation struct {
	id            int64
	appID         int64
	active        bool
	owner         string
	ownerType     string
	permissions   map[string]string
	repositoryIDs map[int64]struct{}
}

type repository struct {
	id             int64
	owner          string
	name           string
	installationID int64
	gitDir         string
}

type pullKey struct {
	repositoryID int64
	number       int
}

type pullRequest struct {
	repositoryID int64
	number       int
	baseRef      string
	headRef      string
	baseSHA      string
	headSHA      string
	state        string
	draft        bool
}

type installationToken struct {
	value          string
	installationID int64
	repositoryIDs  map[int64]struct{}
	permissions    map[string]string
	expiresAt      time.Time
}

type workflowConfig struct {
	repositoryID int64
	id           int64
	name         string
	path         string
	releaseRef   string
}

type activeRun struct {
	command               *exec.Cmd
	cancellationSignalled bool
}

type PendingEvent struct {
	GUID         string          `json:"guid"`
	Event        string          `json:"event"`
	Action       string          `json:"action,omitempty"`
	AppID        int64           `json:"app_id"`
	RepositoryID int64           `json:"repository_id"`
	CreatedAt    time.Time       `json:"created_at"`
	Body         json.RawMessage `json:"body"`
}

type ObservationError struct {
	RepositoryID int64     `json:"repository_id"`
	Operation    string    `json:"operation"`
	Message      string    `json:"message"`
	At           time.Time `json:"at"`
}

type DeliveryAttempt struct {
	GUID             string    `json:"guid"`
	Destination      string    `json:"destination"`
	StatusCode       int       `json:"status_code"`
	InvalidSignature bool      `json:"invalid_signature"`
	AttemptedAt      time.Time `json:"attempted_at"`
	Error            string    `json:"error,omitempty"`
}

type WorkflowRun struct {
	ID                    int64  `json:"id"`
	Attempt               int    `json:"run_attempt"`
	RepositoryID          int64  `json:"repository_id"`
	WorkflowID            int64  `json:"workflow_id"`
	WorkflowName          string `json:"workflow_name"`
	WorkflowPath          string `json:"workflow_path"`
	Event                 string `json:"event"`
	HeadBranch            string `json:"head_branch"`
	HeadSHA               string `json:"head_sha"`
	Status                string `json:"status"`
	Conclusion            string `json:"conclusion,omitempty"`
	CancellationRequested bool   `json:"cancellation_requested"`
	Logs                  string `json:"logs,omitempty"`
}

type UnsupportedRequest struct {
	Method string    `json:"method"`
	Path   string    `json:"path"`
	At     time.Time `json:"at"`
}

type AppInput struct {
	ID            int64  `json:"id"`
	PublicKeyPEM  string `json:"public_key_pem"`
	WebhookURL    string `json:"webhook_url,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

type InstallationInput struct {
	ID          int64             `json:"id"`
	AppID       int64             `json:"app_id"`
	Active      bool              `json:"active"`
	Owner       string            `json:"owner,omitempty"`
	OwnerType   string            `json:"owner_type,omitempty"`
	Permissions map[string]string `json:"permissions"`
}

type RepositoryInput struct {
	ID             int64  `json:"id"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	InstallationID int64  `json:"installation_id"`
}

type PullRequestInput struct {
	RepositoryID int64  `json:"repository_id"`
	Number       int    `json:"number"`
	BaseRef      string `json:"base_ref"`
	HeadRef      string `json:"head_ref"`
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
}

type PullRequestStateInput struct {
	RepositoryID int64  `json:"repository_id"`
	Number       int    `json:"number"`
	State        string `json:"state"`
}

type AdvanceTimeInput struct {
	Duration string `json:"duration"`
}

type WorkflowInput struct {
	RepositoryID int64  `json:"repository_id"`
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ReleaseRef   string `json:"release_ref"`
}

type DeliveryInput struct {
	GUID             string `json:"guid"`
	InvalidSignature bool   `json:"invalid_signature"`
}

type DuplicateEventInput struct {
	GUID string `json:"guid"`
}

type WorkflowTransitionInput struct {
	RunID      int64  `json:"run_id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
}

type ExecuteWorkflowInput struct {
	RunID int64 `json:"run_id"`
}

type StateSnapshot struct {
	Now                 time.Time            `json:"now"`
	Apps                int                  `json:"apps"`
	Installations       int                  `json:"installations"`
	Repositories        int                  `json:"repositories"`
	PullRequests        int                  `json:"pull_requests"`
	Tokens              int                  `json:"tokens"`
	Mutations           uint64               `json:"mutations"`
	UnsupportedRequests []UnsupportedRequest `json:"unsupported_requests"`
	PendingEvents       []PendingEvent       `json:"pending_events"`
	WorkflowRuns        []WorkflowRun        `json:"workflow_runs"`
	ObservationErrors   []ObservationError   `json:"observation_errors"`
	DeliveryAttempts    []DeliveryAttempt    `json:"delivery_attempts"`
}
