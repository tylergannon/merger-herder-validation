package dtu

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v89/github"
	githubserver "github.com/tylergannon/go-github-server"
)

func (w *world) publicHandler() http.Handler {
	api := githubserver.New(githubserver.Services{
		Actions:      actionsService{world: w},
		Apps:         appsService{world: w},
		PullRequests: pullRequestsService{world: w},
	}, nil, githubserver.WithNotFoundCallback(w.recordUnsupported))

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/")
		parts := strings.Split(path, "/")
		if isGitPath(parts) {
			w.serveGit(response, request, parts)
			return
		}
		if hasMalformedNumericPath(request) {
			writeError(response, http.StatusNotFound, "Not Found")
			return
		}
		if isInstallationTokenRequest(request) {
			if err := validateInstallationTokenBody(request); err != nil {
				writeError(response, http.StatusUnprocessableEntity, "Validation Failed")
				return
			}
		}
		if raw, ok := authorizationCredential(request); ok {
			if token, valid := w.authenticateInstallationToken(raw); valid {
				request = request.WithContext(context.WithValue(request.Context(), installationTokenContextKey{}, token))
			}
		}

		unsupported := new(bool)
		request = request.WithContext(context.WithValue(request.Context(), unsupportedRequestKey{}, unsupported))
		buffer := &responseBuffer{header: make(http.Header)}
		api.ServeHTTP(buffer, request)
		if *unsupported {
			response.Header().Set("X-DTU-Unsupported", "true")
			writeError(response, http.StatusNotFound, fmt.Sprintf("Unsupported endpoint: %s %s", request.Method, request.URL.Path))
			return
		}
		if buffer.status >= http.StatusBadRequest {
			var failure struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(buffer.body.Bytes(), &failure); err == nil && failure.Message != "" {
				writeError(response, buffer.status, failure.Message)
				return
			}
		}
		if buffer.header.Get("Content-Type") == "application/json" {
			buffer.header.Set("Content-Type", "application/json; charset=utf-8")
		}
		buffer.flush(response)
	})
}

type actionsService struct {
	githubserver.UnimplementedActionsService
	world *world
}

func (s actionsService) CancelWorkflowRunByID(ctx context.Context, owner, name string, runID int64) (*github.Response, error) {
	token, authOK := installationTokenFromContext(ctx)
	if !authOK {
		return nil, githubAPIError(http.StatusUnauthorized, "Bad credentials")
	}
	s.world.mu.Lock()
	repositoryID, found := s.world.repoNames[repoName(owner, name)]
	if !found {
		s.world.mu.Unlock()
		return nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed || permissionRank(token.permissions["actions"]) < permissionRank("write") {
		s.world.mu.Unlock()
		return nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	index := -1
	for candidate := range s.world.workflowRuns {
		if s.world.workflowRuns[candidate].ID == runID && s.world.workflowRuns[candidate].RepositoryID == repositoryID {
			index = candidate
			break
		}
	}
	if index < 0 {
		s.world.mu.Unlock()
		return nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	if s.world.workflowRuns[index].Status == "completed" {
		s.world.mu.Unlock()
		return nil, githubAPIError(http.StatusConflict, "Cannot cancel a completed workflow run")
	}
	run := s.world.workflowRuns[index]
	run.CancellationRequested = true
	s.world.workflowRuns[index] = run
	active := s.world.activeRuns[runID]
	if active.command != nil && active.command.Process != nil {
		if err := active.command.Process.Signal(os.Interrupt); err == nil {
			active.cancellationSignalled = true
			s.world.activeRuns[runID] = active
		}
	}
	s.world.mutations++
	s.world.mu.Unlock()
	return githubAPIResponse(http.StatusAccepted), nil
}

type appsService struct {
	githubserver.UnimplementedAppsService
	world *world
}

func (s appsService) CreateInstallationToken(_ context.Context, appJWT string, installationID int64, options *github.InstallationTokenOptions) (*github.InstallationToken, *github.Response, error) {
	appID, authOK := s.world.authenticateAppJWT(appJWT)
	if !authOK {
		return nil, nil, githubAPIError(http.StatusUnauthorized, "Bad credentials")
	}
	if options == nil {
		options = new(github.InstallationTokenOptions)
	}
	if len(options.Repositories) > 0 && len(options.RepositoryIDs) > 0 {
		return nil, nil, githubAPIError(http.StatusUnprocessableEntity, "Validation Failed")
	}

	s.world.mu.Lock()
	defer s.world.mu.Unlock()

	installation, found := s.world.installs[installationID]
	if !found || installation.appID != appID {
		return nil, nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	if !installation.active {
		return nil, nil, githubAPIError(http.StatusForbidden, "Installation suspended")
	}

	repositoryIDs, valid := s.world.selectRepositories(installation, *options)
	if !valid {
		return nil, nil, githubAPIError(http.StatusUnprocessableEntity, "Validation Failed")
	}
	permissions, valid := selectPermissions(installation.permissions, options.Permissions)
	if !valid {
		return nil, nil, githubAPIError(http.StatusUnprocessableEntity, "Validation Failed")
	}

	value, err := randomToken()
	if err != nil {
		return nil, nil, githubAPIError(http.StatusInternalServerError, "Internal Server Error")
	}
	expiresAt := s.world.now.Add(tokenLifetime)
	s.world.tokens[value] = installationToken{
		value:          value,
		installationID: installationID,
		repositoryIDs:  copySet(repositoryIDs),
		permissions:    copyMap(permissions),
		expiresAt:      expiresAt,
	}
	s.world.mutations++

	repositories := make([]*github.Repository, 0, len(repositoryIDs))
	for id := range repositoryIDs {
		repo := s.world.repositories[id]
		owner := github.User{Login: new(repo.owner)}
		repositories = append(repositories, &github.Repository{
			ID:       new(repo.id),
			Name:     new(repo.name),
			FullName: new(repo.owner + "/" + repo.name),
			Owner:    &owner,
		})
	}
	wirePermissions := permissionsToGitHub(permissions)
	payload := &github.InstallationToken{
		Token:        new(value),
		ExpiresAt:    &github.Timestamp{Time: expiresAt},
		Permissions:  &wirePermissions,
		Repositories: repositories,
	}
	return payload, githubAPIResponse(http.StatusCreated), nil
}

type pullRequestsService struct {
	githubserver.UnimplementedPullRequestsService
	world *world
}

func (s pullRequestsService) Get(ctx context.Context, owner, name string, number int) (*github.PullRequest, *github.Response, error) {
	token, authOK := installationTokenFromContext(ctx)
	if !authOK {
		return nil, nil, githubAPIError(http.StatusUnauthorized, "Bad credentials")
	}

	s.world.mu.RLock()
	repositoryID, found := s.world.repoNames[repoName(owner, name)]
	if !found {
		s.world.mu.RUnlock()
		return nil, nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed || !canReadPulls(token.permissions) {
		s.world.mu.RUnlock()
		return nil, nil, githubAPIError(http.StatusNotFound, "Not Found")
	}
	repository := s.world.repositories[repositoryID]
	pull, found := s.world.pulls[pullKey{repositoryID: repositoryID, number: number}]
	s.world.mu.RUnlock()
	if !found {
		return nil, nil, githubAPIError(http.StatusNotFound, "Not Found")
	}

	baseSHA := resolveRef(repository.gitDir, pull.baseRef)
	if baseSHA == "" {
		baseSHA = pull.baseSHA
	}
	headSHA := resolveRef(repository.gitDir, pull.headRef)
	if headSHA == "" {
		headSHA = pull.headSHA
	}

	repo := github.Repository{
		ID:       new(repository.id),
		Name:     new(repository.name),
		FullName: new(repository.owner + "/" + repository.name),
		Owner:    &github.User{Login: new(repository.owner)},
	}
	payload := &github.PullRequest{
		Number: new(pull.number),
		State:  new(pull.state),
		Draft:  new(pull.draft),
		Base: &github.PullRequestBranch{
			Ref:  new(pull.baseRef),
			SHA:  new(baseSHA),
			Repo: &repo,
		},
		Head: &github.PullRequestBranch{
			Ref:  new(pull.headRef),
			SHA:  new(headSHA),
			Repo: &repo,
		},
	}
	return payload, githubAPIResponse(http.StatusOK), nil
}

func (w *world) authenticateAppJWT(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	claims := jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("unexpected signing algorithm")
		}
		appID, err := strconv.ParseInt(claims.Issuer, 10, 64)
		if err != nil {
			return nil, errors.New("invalid issuer")
		}
		w.mu.RLock()
		registered, ok := w.apps[appID]
		w.mu.RUnlock()
		if !ok {
			return nil, errors.New("unknown issuer")
		}
		return registered.publicKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}), jwt.WithoutClaimsValidation())
	if err != nil || !parsed.Valid || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return 0, false
	}

	w.mu.RLock()
	now := w.now
	w.mu.RUnlock()
	if claims.IssuedAt.Time.After(now) || !claims.ExpiresAt.Time.After(now) || claims.ExpiresAt.Time.After(now.Add(10*time.Minute)) {
		return 0, false
	}
	appID, err := strconv.ParseInt(claims.Issuer, 10, 64)
	return appID, err == nil
}

func (w *world) authenticateInstallationToken(raw string) (installationToken, bool) {
	w.mu.RLock()
	token, found := w.tokens[raw]
	now := w.now
	w.mu.RUnlock()
	return token, found && now.Before(token.expiresAt)
}

type installationTokenContextKey struct{}

func installationTokenFromContext(ctx context.Context) (installationToken, bool) {
	token, ok := ctx.Value(installationTokenContextKey{}).(installationToken)
	return token, ok
}

func authorizationCredential(request *http.Request) (string, bool) {
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || (!strings.EqualFold(parts[0], "Bearer") && !strings.EqualFold(parts[0], "token")) {
		return "", false
	}
	return parts[1], true
}

func isInstallationTokenRequest(request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	parts := restPathParts(request.URL.Path)
	return len(parts) == 4 && parts[0] == "app" && parts[1] == "installations" && parts[3] == "access_tokens"
}

func hasMalformedNumericPath(request *http.Request) bool {
	parts := restPathParts(request.URL.Path)
	switch {
	case request.Method == http.MethodPost && len(parts) == 4 && parts[0] == "app" && parts[1] == "installations" && parts[3] == "access_tokens":
		_, err := strconv.ParseInt(parts[2], 10, 64)
		return err != nil
	case request.Method == http.MethodGet && len(parts) == 5 && parts[0] == "repos" && parts[3] == "pulls":
		_, err := strconv.Atoi(parts[4])
		return err != nil
	case request.Method == http.MethodPost && len(parts) == 7 && parts[0] == "repos" && parts[3] == "actions" && parts[4] == "runs" && parts[6] == "cancel":
		_, err := strconv.ParseInt(parts[5], 10, 64)
		return err != nil
	default:
		return false
	}
}

func restPathParts(path string) []string {
	if path == "/api/v3" {
		path = "/"
	} else if strings.HasPrefix(path, "/api/v3/") {
		path = strings.TrimPrefix(path, "/api/v3")
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

func validateInstallationTokenBody(request *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20+1))
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > 1<<20 {
		return errors.New("request body exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var options github.InstallationTokenOptions
	err = decoder.Decode(&options)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request contains multiple JSON values")
	}
	return nil
}

func (w *world) selectRepositories(installation installation, options github.InstallationTokenOptions) (map[int64]struct{}, bool) {
	if len(options.RepositoryIDs) == 0 && len(options.Repositories) == 0 {
		return copySet(installation.repositoryIDs), true
	}
	selected := make(map[int64]struct{})
	for _, id := range options.RepositoryIDs {
		if _, allowed := installation.repositoryIDs[id]; !allowed {
			return nil, false
		}
		selected[id] = struct{}{}
	}
	for _, name := range options.Repositories {
		found := false
		for id := range installation.repositoryIDs {
			repository := w.repositories[id]
			if strings.EqualFold(repository.name, name) {
				selected[id] = struct{}{}
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return selected, true
}

func selectPermissions(granted map[string]string, requested *github.InstallationPermissions) (map[string]string, bool) {
	if requested == nil {
		return copyMap(granted), true
	}
	encoded, err := json.Marshal(requested)
	if err != nil {
		return nil, false
	}
	wanted := make(map[string]string)
	if err := json.Unmarshal(encoded, &wanted); err != nil {
		return nil, false
	}
	for permission, level := range wanted {
		grantedLevel, ok := granted[permission]
		if !ok || permissionRank(level) == 0 || permissionRank(level) > permissionRank(grantedLevel) {
			return nil, false
		}
	}
	return wanted, true
}

func permissionsToGitHub(permissions map[string]string) github.InstallationPermissions {
	encoded, _ := json.Marshal(permissions)
	var result github.InstallationPermissions
	_ = json.Unmarshal(encoded, &result)
	return result
}

func permissionRank(level string) int {
	switch level {
	case "read":
		return 1
	case "write":
		return 2
	case "admin":
		return 3
	default:
		return 0
	}
}

func canReadPulls(permissions map[string]string) bool {
	return permissionRank(permissions["pull_requests"]) >= permissionRank("read") ||
		permissionRank(permissions["contents"]) >= permissionRank("read")
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ghs_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func copyMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func copySet(input map[int64]struct{}) map[int64]struct{} {
	result := make(map[int64]struct{}, len(input))
	for value := range input {
		result[value] = struct{}{}
	}
	return result
}

func repoName(owner, name string) string {
	return strings.ToLower(owner) + "/" + strings.ToLower(name)
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("X-GitHub-Api-Version-Selected", "2022-11-28")
	response.Header().Set("X-GitHub-Request-Id", "DTU")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{
		"message":           message,
		"documentation_url": "https://docs.github.com/rest",
		"status":            strconv.Itoa(status),
	})
}

func githubAPIResponse(status int) *github.Response {
	return &github.Response{Response: &http.Response{
		StatusCode: status,
		Header: http.Header{
			"Content-Type":                  []string{"application/json; charset=utf-8"},
			"X-GitHub-Api-Version-Selected": []string{"2022-11-28"},
			"X-GitHub-Request-Id":           []string{"DTU"},
		},
	}}
}

func githubAPIError(status int, message string) error {
	return &github.ErrorResponse{
		Response:         githubAPIResponse(status).Response,
		Message:          message,
		DocumentationURL: "https://docs.github.com/rest",
	}
}

type unsupportedRequestKey struct{}

func (w *world) recordUnsupported(request *http.Request) {
	if unsupported, ok := request.Context().Value(unsupportedRequestKey{}).(*bool); ok {
		*unsupported = true
	}
	w.mu.Lock()
	w.unsupported = append(w.unsupported, UnsupportedRequest{Method: request.Method, Path: request.URL.Path, At: w.now})
	w.mu.Unlock()
}

func (w *world) handleUnsupported(response http.ResponseWriter, request *http.Request) {
	w.recordUnsupported(request)
	response.Header().Set("X-DTU-Unsupported", "true")
	writeError(response, http.StatusNotFound, fmt.Sprintf("Unsupported endpoint: %s %s", request.Method, request.URL.Path))
}

func isGitPath(parts []string) bool {
	if len(parts) < 3 || !strings.HasSuffix(parts[1], ".git") {
		return false
	}
	if len(parts) == 4 && parts[2] == "info" && parts[3] == "refs" {
		return true
	}
	if len(parts) == 3 && (parts[2] == "git-upload-pack" || parts[2] == "git-receive-pack") {
		return true
	}
	return false
}
