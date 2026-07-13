package dtu_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v89/github"
	"github.com/tylergannon/merger-herder-validation/dtu"
)

var proofTime = time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

func TestP1APIGoldenJourney(t *testing.T) {
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	seedAppInstallationRepository(t, control, privateKey, 1, 10, 100, "Acme", "widget")

	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	appClient := githubClient(instance.GitHubURL, appJWT)
	issued, response, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatalf("create installation token: %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create installation token status = %d, want 201", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
	if issued.GetToken() == "" || issued.GetExpiresAt().Time != proofTime.Add(time.Hour) {
		t.Fatalf("unexpected token response: %#v", issued)
	}
	if got := issued.GetPermissions().GetContents(); got != "write" {
		t.Fatalf("contents permission = %q, want write", got)
	}

	fixture := createGitFixture(t)
	remote := authenticatedGitURL(instance.GitURL, "Acme", "widget", issued.GetToken())
	run(t, fixture, "git", "remote", "add", "origin", remote)
	run(t, fixture, "git", "push", "origin", "main", "feature")
	baseSHA := output(t, fixture, "git", "rev-parse", "main")
	headSHA := output(t, fixture, "git", "rev-parse", "feature")
	readOnly, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		RepositoryIDs: []int64{100},
		Permissions:   &github.InstallationPermissions{Contents: new("read")},
	})
	if err != nil {
		t.Fatal(err)
	}
	runFails(t, fixture, "git", "push", authenticatedGitURL(instance.GitURL, "Acme", "widget", readOnly.GetToken()), "main:refs/heads/read-only-proof")

	must(t, control.CreatePullRequest(t.Context(), dtu.PullRequestInput{
		RepositoryID: 100,
		Number:       7,
		BaseRef:      "main",
		HeadRef:      "feature",
		State:        "open",
		Draft:        false,
	}))

	installationClient := githubClient(instance.GitHubURL, issued.GetToken())
	pull, pullResponse, err := installationClient.PullRequests.Get(t.Context(), "aCmE", "WIDGET", 7)
	if err != nil {
		t.Fatalf("get pull request: %v", err)
	}
	if pullResponse.StatusCode != http.StatusOK || pull.GetNumber() != 7 || pull.GetState() != "open" || pull.GetDraft() {
		t.Fatalf("unexpected pull response: status=%d pull=%#v", pullResponse.StatusCode, pull)
	}
	assertBranch(t, "base", pull.GetBase(), "main", baseSHA)
	assertBranch(t, "head", pull.GetHead(), "feature", headSHA)
	assertPermissionAlternatives(t, appClient, instance.GitHubURL)
	assertRawTokenResponse(t, instance.GitHubURL, appJWT)

	beforeRead := state(t, control)
	assertRawPullResponse(t, instance.GitHubURL, issued.GetToken(), headSHA)
	afterRead := state(t, control)
	if afterRead.Mutations != beforeRead.Mutations {
		t.Fatalf("GET mutated state: before=%d after=%d", beforeRead.Mutations, afterRead.Mutations)
	}

	run(t, fixture, "git", "checkout", "feature")
	mustWrite(t, filepath.Join(fixture, "feature.txt"), "moved\n")
	run(t, fixture, "git", "add", "feature.txt")
	run(t, fixture, "git", "commit", "-m", "move feature")
	run(t, fixture, "git", "push", "origin", "feature")
	movedSHA := output(t, fixture, "git", "rev-parse", "feature")
	pull, _, err = installationClient.PullRequests.Get(t.Context(), "Acme", "widget", 7)
	if err != nil {
		t.Fatalf("get moved pull request: %v", err)
	}
	assertBranch(t, "moved head", pull.GetHead(), "feature", movedSHA)

	cloneDir := filepath.Join(t.TempDir(), "clone")
	run(t, "", "git", "clone", "--branch", "feature", remote, cloneDir)
	if cloned := output(t, cloneDir, "git", "rev-parse", "HEAD"); cloned != movedSHA {
		t.Fatalf("cloned SHA = %s, want %s", cloned, movedSHA)
	}

	must(t, control.ChangePullRequestState(t.Context(), dtu.PullRequestStateInput{RepositoryID: 100, Number: 7, State: "closed"}))
	run(t, fixture, "git", "push", "origin", ":feature")
	pull, _, err = installationClient.PullRequests.Get(t.Context(), "Acme", "widget", 7)
	if err != nil {
		t.Fatalf("get closed pull request with deleted source: %v", err)
	}
	if pull.GetState() != "closed" {
		t.Fatalf("state = %q, want closed", pull.GetState())
	}
	assertBranch(t, "retained head", pull.GetHead(), "feature", movedSHA)

	must(t, control.AdvanceTime(t.Context(), dtu.AdvanceTimeInput{Duration: "1h1ns"}))
	_, expiredResponse, err := installationClient.PullRequests.Get(t.Context(), "Acme", "widget", 7)
	assertGitHubError(t, err, expiredResponse, http.StatusUnauthorized)
	runFails(t, fixture, "git", "ls-remote", remote)

	finalState := state(t, control)
	if len(finalState.UnsupportedRequests) != 0 {
		t.Fatalf("golden journey made unsupported requests: %#v", finalState.UnsupportedRequests)
	}
}

func TestP1APINegativeClaims(t *testing.T) {
	instance := startInstance(t)
	control := dtu.NewControlClient(instance.ControlURL)
	key1 := generateKey(t)
	key2 := generateKey(t)
	seedAppInstallationRepository(t, control, key1, 1, 10, 100, "Acme", "widget")
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{ID: 101, Owner: "Acme", Name: "second", InstallationID: 10}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 11, AppID: 1, Active: false, Permissions: map[string]string{"contents": "read"},
	}))
	must(t, control.CreateApp(t.Context(), dtu.AppInput{ID: 2, PublicKeyPEM: publicKeyPEM(t, key2)}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: 20, AppID: 2, Active: true, Permissions: map[string]string{"contents": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{ID: 200, Owner: "Acme", Name: "other", InstallationID: 20}))
	rejectionsBefore := state(t, control)

	tests := []struct {
		name       string
		token      string
		installID  int64
		options    *github.InstallationTokenOptions
		wantStatus int
	}{
		{name: "TOKEN-02 installation token is not App JWT", token: "not-a-jwt", installID: 10, wantStatus: 401},
		{name: "TOKEN-03 wrong signature", token: signAppJWT(t, generateKey(t), 1, proofTime, proofTime.Add(9*time.Minute)), installID: 10, wantStatus: 401},
		{name: "TOKEN-04 wrong algorithm", token: signHMACJWT(t, 1), installID: 10, wantStatus: 401},
		{name: "TOKEN-05 unknown issuer", token: signAppJWT(t, key1, 999, proofTime, proofTime.Add(9*time.Minute)), installID: 10, wantStatus: 401},
		{name: "TOKEN-06 missing iat", token: signClaims(t, key1, jwt.RegisteredClaims{Issuer: "1", ExpiresAt: jwt.NewNumericDate(proofTime.Add(9 * time.Minute))}), installID: 10, wantStatus: 401},
		{name: "TOKEN-06 future iat", token: signAppJWT(t, key1, 1, proofTime.Add(time.Second), proofTime.Add(9*time.Minute)), installID: 10, wantStatus: 401},
		{name: "TOKEN-07 missing expiry", token: signClaims(t, key1, jwt.RegisteredClaims{Issuer: "1", IssuedAt: jwt.NewNumericDate(proofTime)}), installID: 10, wantStatus: 401},
		{name: "TOKEN-07 excessive expiry", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(11*time.Minute)), installID: 10, wantStatus: 401},
		{name: "TOKEN-08 missing installation", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 999, wantStatus: 404},
		{name: "TOKEN-08 foreign installation", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 20, wantStatus: 404},
		{name: "TOKEN-09 inactive installation", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 11, wantStatus: 403},
		{name: "TOKEN-12 both repository selectors", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 10, options: &github.InstallationTokenOptions{Repositories: []string{"widget"}, RepositoryIDs: []int64{100}}, wantStatus: 422},
		{name: "TOKEN-14 unknown repository", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 10, options: &github.InstallationTokenOptions{RepositoryIDs: []int64{999}}, wantStatus: 422},
		{name: "TOKEN-13 permission expansion", token: signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute)), installID: 10, options: &github.InstallationTokenOptions{Permissions: &github.InstallationPermissions{Administration: new("write")}}, wantStatus: 422},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := githubClient(instance.GitHubURL, test.token)
			_, response, err := client.Apps.CreateInstallationToken(t.Context(), test.installID, test.options)
			assertGitHubError(t, err, response, test.wantStatus)
		})
	}
	rejectionsAfter := state(t, control)
	if rejectionsAfter.Mutations != rejectionsBefore.Mutations {
		t.Fatalf("HTTP-05: rejected requests mutated state: before=%d after=%d", rejectionsBefore.Mutations, rejectionsAfter.Mutations)
	}

	validJWT := signAppJWT(t, key1, 1, proofTime, proofTime.Add(9*time.Minute))
	appClient := githubClient(instance.GitHubURL, validJWT)
	first, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.GetToken() == second.GetToken() || first.GetToken() == "" || second.GetToken() == "" {
		t.Fatal("TOKEN-20: tokens are empty or not independent")
	}
	run(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "Acme", "widget", first.GetToken()))
	run(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "Acme", "widget", second.GetToken()))
	run(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "aCmE", "WIDGET", first.GetToken()))

	narrowed, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		Repositories: []string{"widget"},
		Permissions:  &github.InstallationPermissions{Contents: new("read")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(narrowed.Repositories) != 1 || narrowed.Repositories[0].GetName() != "widget" || narrowed.GetPermissions().GetContents() != "read" {
		t.Fatalf("TOKEN-11/13/17: unexpected narrowed token: %#v", narrowed)
	}
	runFails(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "Acme", "second", narrowed.GetToken()))
	run(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "Acme", "widget", narrowed.GetToken()))

	pullOnly, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
		RepositoryIDs: []int64{100},
		Permissions:   &github.InstallationPermissions{PullRequests: new("read")},
	})
	if err != nil {
		t.Fatal(err)
	}
	runFails(t, "", "git", "ls-remote", authenticatedGitURL(instance.GitURL, "Acme", "widget", pullOnly.GetToken()))

	restricted, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{RepositoryIDs: []int64{100}})
	if err != nil {
		t.Fatal(err)
	}
	missing := rawError(t, instance.GitHubURL, restricted.GetToken(), "/repos/Acme/missing/pulls/1")
	unknownPull := rawError(t, instance.GitHubURL, restricted.GetToken(), "/repos/Acme/widget/pulls/999")
	outOfScope := rawError(t, instance.GitHubURL, restricted.GetToken(), "/repos/Acme/other/pulls/1")
	if missing.status != http.StatusNotFound || unknownPull.status != http.StatusNotFound || outOfScope.status != http.StatusNotFound || missing.body != unknownPull.body || missing.body != outOfScope.body {
		t.Fatalf("SCOPE-06/PR-15 mismatch: missing=%#v unknown-pr=%#v out-of-scope=%#v", missing, unknownPull, outOfScope)
	}
	malformedCredential := rawError(t, instance.GitHubURL, "invalid", "/repos/Acme/missing/pulls/1")
	if malformedCredential.status != http.StatusUnauthorized {
		t.Fatalf("HTTP-08/PR-16: malformed credential status=%d, want 401", malformedCredential.status)
	}
	assertRawTokenFailure(t, instance.GitHubURL, validJWT, `{"permissions":{"not_a_permission":"read"}}`, http.StatusUnprocessableEntity)
	assertRawTokenFailure(t, instance.GitHubURL, validJWT, `{}`+"\n{}", http.StatusUnprocessableEntity)

	before := state(t, control)
	request, err := http.NewRequest(http.MethodPut, instance.GitHubURL.ResolveReference(&url.URL{Path: "/repos/Acme/widget/pulls/1"}).String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+restricted.GetToken())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound || response.Header.Get("X-DTU-Unsupported") != "true" {
		t.Fatalf("HTTP-01/06: status=%d unsupported=%q", response.StatusCode, response.Header.Get("X-DTU-Unsupported"))
	}
	after := state(t, control)
	if before.Mutations != after.Mutations || len(after.UnsupportedRequests) != len(before.UnsupportedRequests)+1 {
		t.Fatalf("HTTP-05/06: before=%#v after=%#v", before, after)
	}
	trailingSlash := rawError(t, instance.GitHubURL, restricted.GetToken(), "/repos/Acme/widget/pulls/1/")
	if trailingSlash.status != http.StatusNotFound {
		t.Fatalf("HTTP-01: trailing slash status=%d, want 404", trailingSlash.status)
	}
}

func TestGeneratedRESTContractCompatibility(t *testing.T) {
	instance := startInstance(t)
	wantNotFound := `{"documentation_url":"https://docs.github.com/rest","message":"Not Found","status":"404"}`
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/app/installations/not-a-number/access_tokens"},
		{method: http.MethodGet, path: "/repos/Acme/widget/pulls/not-a-number"},
		{method: http.MethodPost, path: "/repos/Acme/widget/actions/runs/not-a-number/cancel"},
	} {
		t.Run(test.path, func(t *testing.T) {
			failure := rawRequestError(t, instance.GitHubURL, test.method, test.path, "", "")
			if failure.status != http.StatusNotFound || failure.contentType != "application/json; charset=utf-8" || failure.body != wantNotFound {
				t.Fatalf("malformed path response = %#v", failure)
			}
		})
	}

	control := dtu.NewControlClient(instance.ControlURL)
	privateKey := generateKey(t)
	seedAppInstallationRepository(t, control, privateKey, 1, 10, 100, "Acme", "widget")
	appJWT := signAppJWT(t, privateKey, 1, proofTime.Add(-time.Minute), proofTime.Add(9*time.Minute))
	unknownField := rawRequestError(t, instance.GitHubURL, http.MethodPost, "/app/installations/10/access_tokens", appJWT, `{"unknown":true}`)
	bothSelectors := rawRequestError(t, instance.GitHubURL, http.MethodPost, "/app/installations/10/access_tokens", appJWT, `{"repositories":["widget"],"repository_ids":[100]}`)
	if unknownField.status != http.StatusUnprocessableEntity || unknownField.contentType != "application/json; charset=utf-8" || unknownField.body != bothSelectors.body {
		t.Fatalf("validation envelopes differ: unknown-field=%#v both-selectors=%#v", unknownField, bothSelectors)
	}
}

func TestExecutableStartup(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "dtu-github")
	run(t, "..", "go", "build", "-o", binary, "./cmd/dtu-github")
	command := exec.Command(binary, "-initial-time", proofTime.Format(time.RFC3339))
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("read readiness: %v", scanner.Err())
	}
	var ready struct {
		GitHubURL  string `json:"github_url"`
		ControlURL string `json:"control_url"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &ready); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	controlURL, err := url.Parse(ready.ControlURL)
	if err != nil {
		t.Fatal(err)
	}
	control := dtu.NewControlClient(*controlURL)
	snapshot := state(t, control)
	if snapshot.Now != proofTime || ready.GitHubURL == "" {
		t.Fatalf("unexpected readiness/state: ready=%#v state=%#v", ready, snapshot)
	}
	publicResponse, err := http.Get(ready.GitHubURL + "unsupported")
	if err != nil {
		t.Fatalf("call subprocess public listener: %v", err)
	}
	publicResponse.Body.Close()
	if publicResponse.StatusCode != http.StatusNotFound || publicResponse.Header.Get("X-DTU-Unsupported") != "true" {
		t.Fatalf("subprocess public listener: status=%d unsupported=%q", publicResponse.StatusCode, publicResponse.Header.Get("X-DTU-Unsupported"))
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("server shutdown: %v", err)
	}
}

func startInstance(t *testing.T) dtu.Instance {
	t.Helper()
	instance, err := dtu.Start(dtu.Config{InitialTime: proofTime})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := instance.Close(ctx); err != nil {
			t.Errorf("close server: %v", err)
		}
	})
	return instance
}

func seedAppInstallationRepository(t *testing.T, control dtu.ControlClient, key *rsa.PrivateKey, appID, installationID, repositoryID int64, owner, name string) {
	t.Helper()
	must(t, control.CreateApp(t.Context(), dtu.AppInput{ID: appID, PublicKeyPEM: publicKeyPEM(t, key)}))
	must(t, control.CreateInstallation(t.Context(), dtu.InstallationInput{
		ID: installationID, AppID: appID, Active: true,
		Permissions: map[string]string{"contents": "write", "pull_requests": "read"},
	}))
	must(t, control.CreateRepository(t.Context(), dtu.RepositoryInput{
		ID: repositoryID, Owner: owner, Name: name, InstallationID: installationID,
	}))
}

func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func publicKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	encoded, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}))
}

func signAppJWT(t *testing.T, key *rsa.PrivateKey, appID int64, issuedAt, expiresAt time.Time) string {
	t.Helper()
	return signClaims(t, key, jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(appID, 10),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})
}

func signClaims(t *testing.T, key *rsa.PrivateKey, claims jwt.RegisteredClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func signHMACJWT(t *testing.T, appID int64) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(appID, 10),
		IssuedAt:  jwt.NewNumericDate(proofTime),
		ExpiresAt: jwt.NewNumericDate(proofTime.Add(9 * time.Minute)),
	})
	signed, err := token.SignedString([]byte("not-an-rsa-key"))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func githubClient(baseURL url.URL, token string) *github.Client {
	client, err := github.NewClient(
		github.WithAuthToken(token),
		github.WithEnterpriseURLs(baseURL.String(), baseURL.String()),
	)
	if err != nil {
		panic(err)
	}
	return client
}

func createGitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-b", "main")
	run(t, dir, "git", "config", "user.name", "DTU Test")
	run(t, dir, "git", "config", "user.email", "dtu@example.test")
	mustWrite(t, filepath.Join(dir, "README.md"), "base\n")
	run(t, dir, "git", "add", "README.md")
	run(t, dir, "git", "commit", "-m", "base")
	run(t, dir, "git", "checkout", "-b", "feature")
	mustWrite(t, filepath.Join(dir, "README.md"), "feature\n")
	run(t, dir, "git", "commit", "-am", "feature")
	return dir
}

func authenticatedGitURL(base url.URL, owner, name, token string) string {
	base.User = url.UserPassword("x-access-token", token)
	base.Path = "/" + owner + "/" + name + ".git"
	return base.String()
}

func assertPermissionAlternatives(t *testing.T, appClient *github.Client, baseURL url.URL) {
	t.Helper()
	tests := []struct {
		name        string
		permissions github.InstallationPermissions
	}{
		{name: "PR-05 contents read", permissions: github.InstallationPermissions{Contents: new("read")}},
		{name: "PR-05 pull requests read", permissions: github.InstallationPermissions{PullRequests: new("read")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issued, _, err := appClient.Apps.CreateInstallationToken(t.Context(), 10, &github.InstallationTokenOptions{
				RepositoryIDs: []int64{100},
				Permissions:   &test.permissions,
			})
			if err != nil {
				t.Fatal(err)
			}
			client := githubClient(baseURL, issued.GetToken())
			if _, _, err := client.PullRequests.Get(t.Context(), "Acme", "widget", 7); err != nil {
				t.Fatalf("permission alternative rejected: %v", err)
			}
		})
	}
}

func assertBranch(t *testing.T, label string, branch *github.PullRequestBranch, ref, sha string) {
	t.Helper()
	if branch.GetRef() != ref || branch.GetSHA() != sha || branch.GetRepo().GetFullName() != "Acme/widget" {
		t.Fatalf("%s = %#v, want ref=%s sha=%s repo=Acme/widget", label, branch, ref, sha)
	}
}

func assertRawPullResponse(t *testing.T, baseURL url.URL, token, headSHA string) {
	t.Helper()
	for _, scheme := range []string{"Bearer", "token"} {
		t.Run(scheme, func(t *testing.T) {
			endpoint := baseURL.ResolveReference(&url.URL{Path: "/repos/Acme/widget/pulls/7"})
			request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", scheme+" "+token)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var body struct {
				Number int `json:"number"`
				Head   struct {
					SHA string `json:"sha"`
				} `json:"head"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || body.Number != 7 || body.Head.SHA != headSHA {
				t.Fatalf("raw response: status=%d body=%#v", response.StatusCode, body)
			}
		})
	}
}

func assertRawTokenResponse(t *testing.T, baseURL url.URL, appJWT string) {
	t.Helper()
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/app/installations/10/access_tokens"})
	request, err := http.NewRequest(http.MethodPost, endpoint.String(), strings.NewReader(`{"repository_ids":[100],"permissions":{"contents":"read"}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+appJWT)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct {
		Token        string            `json:"token"`
		ExpiresAt    time.Time         `json:"expires_at"`
		Permissions  map[string]string `json:"permissions"`
		Repositories []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"repositories"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated || body.Token == "" || body.ExpiresAt != proofTime.Add(time.Hour) || body.Permissions["contents"] != "read" || len(body.Repositories) != 1 || body.Repositories[0].ID != 100 || body.Repositories[0].Name != "widget" {
		t.Fatalf("raw token response: status=%d body=%#v", response.StatusCode, body)
	}
}

type rawFailure struct {
	status      int
	contentType string
	body        string
}

func rawError(t *testing.T, baseURL url.URL, token, path string) rawFailure {
	t.Helper()
	return rawRequestError(t, baseURL, http.MethodGet, path, token, "")
}

func rawRequestError(t *testing.T, baseURL url.URL, method, path, token, requestBody string) rawFailure {
	t.Helper()
	request, err := http.NewRequest(method, baseURL.ResolveReference(&url.URL{Path: path}).String(), strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if requestBody != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(body)
	return rawFailure{status: response.StatusCode, contentType: response.Header.Get("Content-Type"), body: string(encoded)}
}

func assertRawTokenFailure(t *testing.T, baseURL url.URL, token, body string, status int) {
	t.Helper()
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/app/installations/10/access_tokens"})
	request, err := http.NewRequest(http.MethodPost, endpoint.String(), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("raw token status=%d, want %d", response.StatusCode, status)
	}
	var githubError github.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&githubError); err != nil || githubError.Message == "" {
		t.Fatalf("decode raw token error: error=%v response=%#v", err, githubError)
	}
}

func assertGitHubError(t *testing.T, err error, response *github.Response, status int) {
	t.Helper()
	if err == nil || response == nil || response.StatusCode != status {
		t.Fatalf("error=%v response=%#v, want status %d", err, response, status)
	}
	var githubError *github.ErrorResponse
	if !errors.As(err, &githubError) || githubError.Message == "" {
		t.Fatalf("error %T %v is not github.ErrorResponse", err, err)
	}
}

func state(t *testing.T, control dtu.ControlClient) dtu.StateSnapshot {
	t.Helper()
	snapshot, err := control.State(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}

func runFails(t *testing.T, dir, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("%s %s unexpectedly succeeded\n%s", name, strings.Join(arguments, " "), output)
	}
}

func output(t *testing.T, dir, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = dir
	value, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, value)
	}
	return strings.TrimSpace(string(value))
}

func ExampleInstance() {
	instance, err := dtu.Start(dtu.Config{InitialTime: proofTime})
	if err != nil {
		panic(err)
	}
	fmt.Println(instance.GitHubURL.Scheme)
	_ = instance.Close(context.Background())
	// Output: http
}
