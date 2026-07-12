package dtu_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/tylergannon/merger-herder-validation/dtu"
)

const (
	p1APIToken    = "p1-api-token"
	p1WorkerToken = "p1-worker-token"
	p1Secret      = "p1-webhook-secret"
)

func TestMergeHerderP1OneCleanPRLands(t *testing.T) {
	productDir := requireP1SystemRuntime(t)
	t.Setenv("DTU_REQUIRE_ACT", "1")
	requireActRuntime(t)
	workerCalls := make(chan p1WorkerCall, 2)
	worker := newP1Worker(t, workerCalls)
	databaseURL := startP1Postgres(t)
	migrateP1Database(t, productDir, databaseURL)

	productPort := freePort(t)
	productURL := "http://127.0.0.1:" + strconv.Itoa(productPort)
	initialTime := time.Now().UTC().Truncate(time.Second)
	instance, err := dtu.Start(dtu.Config{InitialTime: initialTime})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := instance.Close(ctx); err != nil {
			t.Errorf("close DTU: %v", err)
		}
	})
	privateKey := generateKey(t)
	product := startMergeHerder(t, productDir, productPort, databaseURL, instance, worker.URL, privateKey)

	control := dtu.NewControlClient(instance.ControlURL)
	must(t, control.CreateApp(t.Context(), dtu.AppInput{
		ID: 1, PublicKeyPEM: publicKeyPEM(t, privateKey),
		WebhookURL: productURL + "/api/v1/github-webhook", WebhookSecret: p1Secret,
	}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 10, AppID: 1, Active: true, Owner: "Acme", OwnerType: "Organization",
		Permissions: map[string]string{"actions": "write", "contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{
		ID: 100, Owner: "Acme", Name: "widget", InstallationID: 10,
	}))
	issuedToken := mintP1FixtureToken(t, instance, privateKey, initialTime)
	fixture, baseSHA, sourceSHA, sourceTree := createP1SystemFixture(t)
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issuedToken)
	run(t, fixture, "git", "push", remote, "main:refs/heads/main", "feature:refs/heads/feature")
	must(t, control.CreatePullRequest(t.Context(), dtu.PullRequestInput{
		RepositoryID: 100, Number: 7, BaseRef: "main", HeadRef: "feature", State: "open",
	}))
	must(t, control.ConfigureWorkflow(t.Context(), dtu.WorkflowInput{
		RepositoryID: 100, ID: 77, Name: "CI", Path: ".github/workflows/ci.yml", ReleaseRef: "R",
	}))

	delivered := make(map[string]struct{})
	deliverMatchingEvents(t, control, delivered, func(event dtu.PendingEvent) bool {
		return event.Event == "installation" || event.Event == "installation_repositories" || event.Event == "push"
	})

	submission := submitP1(t, productURL, product)
	if submission.Status != "awaiting_ci" || submission.ReleaseRef != "R" || submission.ReleaseSHA == "" {
		t.Fatalf("unexpected submission: %#v\nproduct logs:\n%s", submission, product.String())
	}
	select {
	case call := <-workerCalls:
		if call.BaseSHA != baseSHA || call.SourceSHA != sourceSHA {
			t.Fatalf("worker call = %#v, want base=%s source=%s", call, baseSHA, sourceSHA)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MergeHerder did not call worker")
	}

	deliverMatchingEvents(t, control, delivered, func(event dtu.PendingEvent) bool {
		return event.Event == "push" || (event.Event == "workflow_run" && event.Action == "requested")
	})
	createdRun := latestWorkflowRun(t, control)
	execution := make(chan error, 1)
	go func() {
		execution <- control.ExecuteWorkflowRun(t.Context(), dtu.ExecuteWorkflowInput{RunID: createdRun.ID})
	}()
	waitForWorkflowStatus(t, control, createdRun.ID, "in_progress")
	deliverMatchingEvents(t, control, delivered, func(event dtu.PendingEvent) bool {
		return event.Event == "workflow_run" && event.Action == "in_progress"
	})
	select {
	case err := <-execution:
		must(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("real workflow execution did not complete")
	}
	deliverMatchingEvents(t, control, delivered, func(event dtu.PendingEvent) bool {
		return event.Event == "workflow_run" && event.Action == "completed"
	})
	deliverMatchingEvents(t, control, delivered, func(event dtu.PendingEvent) bool {
		return event.Event == "push" && pushRef(t, event) == "refs/heads/main"
	})

	batch := observeP1(t, productURL, product)
	if batch.Status != "completed" || batch.ReleaseSHA != submission.ReleaseSHA || batch.LandedSHA != submission.ReleaseSHA || batch.WorkflowRunID != strconv.FormatInt(createdRun.ID, 10) || batch.WorkflowRunAttempt != createdRun.Attempt {
		t.Fatalf("unexpected completed batch: %#v\nproduct logs:\n%s", batch, product.String())
	}
	assertP1GitResult(t, remote, baseSHA, sourceSHA, sourceTree, submission.ReleaseSHA)
	finalRun := workflowRun(t, control, createdRun.ID)
	if finalRun.Status != "completed" || finalRun.Conclusion != "success" || finalRun.HeadSHA != submission.ReleaseSHA {
		t.Fatalf("unexpected authoritative workflow: %#v", finalRun)
	}
	finalState := state(t, control)
	if len(finalState.UnsupportedRequests) != 0 || len(finalState.ObservationErrors) != 0 {
		t.Fatalf("DTU diagnostics are not clean: unsupported=%#v observations=%#v", finalState.UnsupportedRequests, finalState.ObservationErrors)
	}
}

type p1Submission struct {
	BatchID    string `json:"batchId"`
	Status     string `json:"status"`
	ReleaseRef string `json:"releaseRef"`
	ReleaseSHA string `json:"releaseSha"`
}

type p1Batch struct {
	Status             string `json:"status"`
	ReleaseSHA         string `json:"releaseSha"`
	LandedSHA          string `json:"landedSha"`
	WorkflowRunID      string `json:"workflowRunId"`
	WorkflowRunAttempt int    `json:"workflowRunAttempt"`
}

func submitP1(t *testing.T, productURL string, productLogs *lockedBuffer) p1Submission {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, productURL+"/api/v1/queue", strings.NewReader(`{"owner":"Acme","repository":"widget","pullNumber":7}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+p1APIToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("queue submission returned %s: %s\nproduct logs:\n%s", response.Status, body, productLogs.String())
	}
	var result p1Submission
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func observeP1(t *testing.T, productURL string, productLogs *lockedBuffer) p1Batch {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, productURL+"/api/v1/queue/Acme/widget", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+p1APIToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result p1Batch
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("queue observation returned %s: %s\nproduct logs:\n%s", response.Status, body, productLogs.String())
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func mintP1FixtureToken(t *testing.T, instance dtu.Instance, privateKey *rsa.PrivateKey, now time.Time) string {
	t.Helper()
	jwt := signAppJWT(t, privateKey, 1, now.Add(-time.Minute), now.Add(8*time.Minute))
	issued, _, err := githubClient(instance.GitHubURL, jwt).Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	return issued.GetToken()
}

func createP1SystemFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	directory := t.TempDir()
	run(t, directory, "git", "init", "-b", "main")
	run(t, directory, "git", "config", "user.name", "P1 Fixture")
	run(t, directory, "git", "config", "user.email", "p1@example.test")
	mustWrite(t, filepath.Join(directory, "README.md"), "base\n")
	run(t, directory, "git", "add", "README.md")
	run(t, directory, "git", "commit", "-m", "base")
	baseSHA := output(t, directory, "git", "rev-parse", "HEAD")
	run(t, directory, "git", "checkout", "-b", "feature")
	mustWrite(t, filepath.Join(directory, "README.md"), "feature\n")
	must(t, os.MkdirAll(filepath.Join(directory, ".github", "workflows"), 0o755))
	mustWrite(t, filepath.Join(directory, ".github", "workflows", "ci.yml"), `name: CI
on: push
jobs:
  prove:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5
      - name: Prove merged source tree
        run: test "$(cat README.md)" = "feature"
`)
	run(t, directory, "git", "add", "README.md", ".github/workflows/ci.yml")
	run(t, directory, "git", "commit", "-m", "feature")
	sourceSHA := output(t, directory, "git", "rev-parse", "HEAD")
	sourceTree := output(t, directory, "git", "rev-parse", "HEAD^{tree}")
	return directory, baseSHA, sourceSHA, sourceTree
}

func deliverMatchingEvents(t *testing.T, control dtu.ControlClient, delivered map[string]struct{}, matches func(dtu.PendingEvent) bool) {
	t.Helper()
	for {
		deliveredOne := false
		for _, event := range state(t, control).PendingEvents {
			if _, found := delivered[event.GUID]; found || !matches(event) {
				continue
			}
			must(t, control.DeliverEvent(t.Context(), dtu.DeliveryInput{GUID: event.GUID}))
			delivered[event.GUID] = struct{}{}
			deliveredOne = true
		}
		if !deliveredOne {
			return
		}
	}
}

func latestWorkflowRun(t *testing.T, control dtu.ControlClient) dtu.WorkflowRun {
	t.Helper()
	runs := state(t, control).WorkflowRuns
	if len(runs) == 0 {
		t.Fatal("DTU created no workflow run")
	}
	return runs[len(runs)-1]
}

func pushRef(t *testing.T, event dtu.PendingEvent) string {
	t.Helper()
	var payload struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(event.Body, &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Ref
}

func assertP1GitResult(t *testing.T, remote, baseSHA, sourceSHA, sourceTree, releaseSHA string) {
	t.Helper()
	directory := t.TempDir()
	run(t, directory, "git", "init")
	run(t, directory, "git", "fetch", "--no-tags", remote,
		"refs/heads/main:refs/assert/main",
		"refs/heads/feature:refs/assert/feature",
		"refs/heads/R:refs/assert/release",
	)
	mainSHA := output(t, directory, "git", "rev-parse", "refs/assert/main")
	actualReleaseSHA := output(t, directory, "git", "rev-parse", "refs/assert/release")
	actualSourceSHA := output(t, directory, "git", "rev-parse", "refs/assert/feature")
	parentSHA := output(t, directory, "git", "rev-parse", "refs/assert/main^")
	mainTree := output(t, directory, "git", "rev-parse", "refs/assert/main^{tree}")
	remoteSourceTree := output(t, directory, "git", "rev-parse", "refs/assert/feature^{tree}")
	commitCount := output(t, directory, "git", "rev-list", "--count", baseSHA+"..refs/assert/main")
	if mainSHA != releaseSHA || actualReleaseSHA != releaseSHA || actualSourceSHA != sourceSHA || parentSHA != baseSHA || mainTree != sourceTree || remoteSourceTree != sourceTree || commitCount != "1" {
		t.Fatalf("unexpected P1 Git result: main=%s release=%s source=%s parent=%s mainTree=%s sourceTree=%s remoteSourceTree=%s count=%s", mainSHA, actualReleaseSHA, actualSourceSHA, parentSHA, mainTree, sourceTree, remoteSourceTree, commitCount)
	}
}

func requireP1SystemRuntime(t *testing.T) string {
	t.Helper()
	directory := os.Getenv("MERGE_HERDER_DIR")
	if directory == "" {
		if os.Getenv("DTU_REQUIRE_SYSTEM") == "1" {
			t.Fatal("MERGE_HERDER_DIR is not set")
		}
		t.Skip("MERGE_HERDER_DIR is not set")
	}
	for _, command := range []string{"vp", "docker", "migrate"} {
		if _, err := exec.LookPath(command); err != nil {
			if os.Getenv("DTU_REQUIRE_SYSTEM") == "1" {
				t.Fatalf("%s is not installed", command)
			}
			t.Skipf("%s is not installed", command)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "package.json")); err != nil {
		t.Fatalf("invalid MERGE_HERDER_DIR: %v", err)
	}
	return directory
}

func startP1Postgres(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	name := fmt.Sprintf("merge-herder-p1-%d", time.Now().UnixNano())
	runCommand(t, "", nil, "docker", "run", "--detach", "--rm", "--name", name,
		"-e", "POSTGRES_USER=root", "-e", "POSTGRES_PASSWORD=mysecretpassword", "-e", "POSTGRES_DB=p1",
		"-p", fmt.Sprintf("127.0.0.1:%d:5432", port), "postgres:17-alpine")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("docker", "exec", name, "pg_isready", "-U", "root", "-d", "p1").Run() == nil {
			return fmt.Sprintf("postgres://root:mysecretpassword@127.0.0.1:%d/p1?sslmode=disable", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Postgres did not become ready")
	return ""
}

func migrateP1Database(t *testing.T, productDir, databaseURL string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastOutput []byte
	var lastErr error
	for time.Now().Before(deadline) {
		command := exec.Command("migrate", "-path", "model/migrations", "-database", databaseURL, "up")
		command.Dir = productDir
		lastOutput, lastErr = command.CombinedOutput()
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("migrate database: %v\n%s", lastErr, lastOutput)
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(value)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func startMergeHerder(t *testing.T, directory string, port int, databaseURL string, instance dtu.Instance, workerURL string, privateKey *rsa.PrivateKey) *lockedBuffer {
	t.Helper()
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	environment := append(os.Environ(),
		"DATABASE_URL="+databaseURL,
		fmt.Sprintf("ORIGIN=http://127.0.0.1:%d", port),
		"BETTER_AUTH_SECRET=01234567890123456789012345678901",
		"GITHUB_APP_CLIENT_ID=p1-client",
		"GITHUB_APP_CLIENT_SECRET=p1-client-secret",
		"GITHUB_OAUTH_SERVICE_URL="+instance.GitHubURL.String(),
		"GITHUB_API_URL="+instance.GitHubURL.String(),
		"GITHUB_GIT_URL="+instance.GitURL.String(),
		"GITHUB_APP_WEBHOOK_SECRET="+p1Secret,
		"GITHUB_APP_PRIVATE_KEY_B64="+base64.StdEncoding.EncodeToString(privatePEM),
		"GITHUB_APP_ID=1",
		"GITHUB_APP_OWNER_TYPE=organization",
		"GITHUB_APP_OWNER=Acme",
		"MERGE_HERDER_API_TOKEN="+p1APIToken,
		"MERGE_HERDER_WORKER_URL="+workerURL,
		"MERGE_HERDER_WORKER_TOKEN="+p1WorkerToken,
		"MERGE_HERDER_RELEASE_REF=R",
		"MERGE_HERDER_WORKFLOW_ID=77",
	)
	command := exec.Command("vp", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	command.Dir = directory
	command.Env = environment
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logs := new(lockedBuffer)
	command.Stdout = logs
	command.Stderr = logs
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = command.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		if command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGINT)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
	})
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/api/v1/queue/Acme/widget", port)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusUnauthorized {
				return logs
			}
		}
		select {
		case <-done:
			t.Fatalf("MergeHerder exited before readiness: %v\n%s", waitErr, logs.String())
		default:
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("MergeHerder did not become ready\n%s", logs.String())
	return logs
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func runCommand(t *testing.T, directory string, environment []string, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	if directory != "" {
		command.Dir = directory
	}
	if environment != nil {
		command.Env = environment
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

type p1WorkerCall struct {
	BaseSHA   string
	SourceSHA string
}

func newP1Worker(t *testing.T, calls chan<- p1WorkerCall) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+p1WorkerToken {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input struct {
			Version   int    `json:"version"`
			Bundle    string `json:"bundle"`
			BaseSHA   string `json:"baseSha"`
			SourceSHA string `json:"sourceSha"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Version != 1 {
			http.Error(response, "invalid input", http.StatusBadRequest)
			return
		}
		bundle, err := base64.StdEncoding.DecodeString(input.Bundle)
		if err != nil {
			http.Error(response, "invalid bundle", http.StatusBadRequest)
			return
		}
		directory, err := os.MkdirTemp("", "p1-worker-")
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(directory)
		bundlePath := filepath.Join(directory, "input.bundle")
		if err := os.WriteFile(bundlePath, bundle, 0o600); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := workerGit(directory, "init"); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := workerGit(directory, "fetch", bundlePath, "refs/worker/base:refs/worker/base", "refs/worker/head:refs/worker/head"); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		base, err := workerGit(directory, "rev-parse", "refs/worker/base")
		if err != nil || base != input.BaseSHA {
			http.Error(response, "base identity mismatch", http.StatusBadRequest)
			return
		}
		head, err := workerGit(directory, "rev-parse", "refs/worker/head")
		if err != nil || head != input.SourceSHA {
			http.Error(response, "source identity mismatch", http.StatusBadRequest)
			return
		}
		patch, err := workerGitRaw(directory, "diff", "--binary", input.BaseSHA, input.SourceSHA)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		calls <- p1WorkerCall{BaseSHA: base, SourceSHA: head}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{
			"patch": patch, "commitMessage": "Squash PR #7",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func workerGit(directory string, arguments ...string) (string, error) {
	output, err := workerGitRaw(directory, arguments...)
	return strings.TrimSpace(output), err
}

func workerGitRaw(directory string, arguments ...string) (string, error) {
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, output)
	}
	return string(output), nil
}
