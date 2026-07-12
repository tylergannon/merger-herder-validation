package dtu

import (
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
	"github.com/google/go-github/v81/github"
)

func (w *world) publicHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/")
		parts := strings.Split(path, "/")

		switch {
		case len(parts) == 4 && parts[0] == "app" && parts[1] == "installations" && parts[3] == "access_tokens":
			if request.Method != http.MethodPost {
				w.handleUnsupported(response, request)
				return
			}
			w.createInstallationToken(response, request, parts[2])
		case len(parts) == 5 && parts[0] == "repos" && parts[3] == "pulls":
			if request.Method != http.MethodGet {
				w.handleUnsupported(response, request)
				return
			}
			w.getPullRequest(response, request, parts[1], parts[2], parts[4])
		case len(parts) == 7 && parts[0] == "repos" && parts[3] == "actions" && parts[4] == "runs" && parts[6] == "cancel":
			if request.Method != http.MethodPost {
				w.handleUnsupported(response, request)
				return
			}
			w.cancelWorkflowRun(response, request, parts[1], parts[2], parts[5])
		case isGitPath(parts):
			w.serveGit(response, request, parts)
		default:
			w.handleUnsupported(response, request)
		}
	})
}

func (w *world) cancelWorkflowRun(response http.ResponseWriter, request *http.Request, owner, name, rawRunID string) {
	runID, err := strconv.ParseInt(rawRunID, 10, 64)
	if err != nil {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	token, authOK := w.authenticateInstallationToken(request)
	if !authOK {
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}
	w.mu.Lock()
	repositoryID, found := w.repoNames[repoName(owner, name)]
	if !found {
		w.mu.Unlock()
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed || permissionRank(token.permissions["actions"]) < permissionRank("write") {
		w.mu.Unlock()
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	index := -1
	for candidate := range w.workflowRuns {
		if w.workflowRuns[candidate].ID == runID && w.workflowRuns[candidate].RepositoryID == repositoryID {
			index = candidate
			break
		}
	}
	if index < 0 {
		w.mu.Unlock()
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if w.workflowRuns[index].Status == "completed" {
		w.mu.Unlock()
		writeError(response, http.StatusConflict, "Cannot cancel a completed workflow run")
		return
	}
	run := w.workflowRuns[index]
	run.CancellationRequested = true
	w.workflowRuns[index] = run
	active := w.activeRuns[runID]
	if active.command != nil && active.command.Process != nil {
		if err := active.command.Process.Signal(os.Interrupt); err == nil {
			active.cancellationSignalled = true
			w.activeRuns[runID] = active
		}
	}
	w.mutations++
	w.mu.Unlock()
	response.Header().Set("X-GitHub-Request-Id", "DTU")
	response.WriteHeader(http.StatusAccepted)
}

func (w *world) createInstallationToken(response http.ResponseWriter, request *http.Request, rawID string) {
	installationID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}

	appID, authOK := w.authenticateAppJWT(request)
	if !authOK {
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}

	var options github.InstallationTokenOptions
	if err := decodeOptionalJSON(request, &options); err != nil {
		writeError(response, http.StatusUnprocessableEntity, "Validation Failed")
		return
	}
	if len(options.Repositories) > 0 && len(options.RepositoryIDs) > 0 {
		writeError(response, http.StatusUnprocessableEntity, "Validation Failed")
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	installation, found := w.installs[installationID]
	if !found || installation.appID != appID {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if !installation.active {
		writeError(response, http.StatusForbidden, "Installation suspended")
		return
	}

	repositoryIDs, valid := w.selectRepositories(installation, options)
	if !valid {
		writeError(response, http.StatusUnprocessableEntity, "Validation Failed")
		return
	}
	permissions, valid := selectPermissions(installation.permissions, options.Permissions)
	if !valid {
		writeError(response, http.StatusUnprocessableEntity, "Validation Failed")
		return
	}

	value, err := randomToken()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	expiresAt := w.now.Add(tokenLifetime)
	w.tokens[value] = installationToken{
		value:          value,
		installationID: installationID,
		repositoryIDs:  copySet(repositoryIDs),
		permissions:    copyMap(permissions),
		expiresAt:      expiresAt,
	}
	w.mutations++

	repositories := make([]*github.Repository, 0, len(repositoryIDs))
	for id := range repositoryIDs {
		repo := w.repositories[id]
		owner := github.User{Login: new(repo.owner)}
		repositories = append(repositories, &github.Repository{
			ID:       new(repo.id),
			Name:     new(repo.name),
			FullName: new(repo.owner + "/" + repo.name),
			Owner:    &owner,
		})
	}
	wirePermissions := permissionsToGitHub(permissions)
	payload := github.InstallationToken{
		Token:        new(value),
		ExpiresAt:    &github.Timestamp{Time: expiresAt},
		Permissions:  &wirePermissions,
		Repositories: repositories,
	}
	writeJSON(response, http.StatusCreated, payload)
}

func (w *world) getPullRequest(response http.ResponseWriter, request *http.Request, owner, name, rawNumber string) {
	number, err := strconv.Atoi(rawNumber)
	if err != nil {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}

	token, authOK := w.authenticateInstallationToken(request)
	if !authOK {
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}

	w.mu.RLock()
	repositoryID, found := w.repoNames[repoName(owner, name)]
	if !found {
		w.mu.RUnlock()
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed || !canReadPulls(token.permissions) {
		w.mu.RUnlock()
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	repository := w.repositories[repositoryID]
	pull, found := w.pulls[pullKey{repositoryID: repositoryID, number: number}]
	w.mu.RUnlock()
	if !found {
		writeError(response, http.StatusNotFound, "Not Found")
		return
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
	payload := github.PullRequest{
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
	writeJSON(response, http.StatusOK, payload)
}

func (w *world) authenticateAppJWT(request *http.Request) (int64, bool) {
	raw, found := bearerToken(request)
	if !found {
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

func (w *world) authenticateInstallationToken(request *http.Request) (installationToken, bool) {
	scheme, raw, found := strings.Cut(request.Header.Get("Authorization"), " ")
	found = found && raw != "" && (strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "token"))
	if !found {
		return installationToken{}, false
	}
	w.mu.RLock()
	token, found := w.tokens[raw]
	now := w.now
	w.mu.RUnlock()
	return token, found && now.Before(token.expiresAt)
}

func bearerToken(request *http.Request) (string, bool) {
	scheme, value, found := strings.Cut(request.Header.Get("Authorization"), " ")
	return value, found && strings.EqualFold(scheme, "Bearer") && value != ""
}

func decodeOptionalJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
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
	return base64.RawURLEncoding.EncodeToString(bytes), nil
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

func (w *world) handleUnsupported(response http.ResponseWriter, request *http.Request) {
	w.mu.Lock()
	w.unsupported = append(w.unsupported, UnsupportedRequest{Method: request.Method, Path: request.URL.Path, At: w.now})
	w.mu.Unlock()
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
