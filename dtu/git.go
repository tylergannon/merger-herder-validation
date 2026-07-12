package dtu

import (
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (w *world) serveGit(response http.ResponseWriter, request *http.Request, parts []string) {
	service, methodOK := gitService(request, parts)
	if !methodOK {
		w.handleUnsupported(response, request)
		return
	}
	username, rawToken, authOK := request.BasicAuth()
	if !authOK || username != "x-access-token" {
		response.Header().Set("WWW-Authenticate", `Basic realm="GitHub"`)
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}

	w.mu.RLock()
	token, found := w.tokens[rawToken]
	now := w.now
	repositoryID, repoFound := w.repoNames[repoName(parts[0], strings.TrimSuffix(parts[1], ".git"))]
	repository := w.repositories[repositoryID]
	w.mu.RUnlock()
	if !found || !now.Before(token.expiresAt) {
		writeError(response, http.StatusUnauthorized, "Bad credentials")
		return
	}
	if !repoFound {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if _, allowed := token.repositoryIDs[repositoryID]; !allowed {
		writeError(response, http.StatusNotFound, "Not Found")
		return
	}
	if service == "git-receive-pack" && permissionRank(token.permissions["contents"]) < permissionRank("write") {
		writeError(response, http.StatusForbidden, "Resource not accessible by integration")
		return
	}
	if service == "git-upload-pack" && permissionRank(token.permissions["contents"]) < permissionRank("read") {
		writeError(response, http.StatusForbidden, "Resource not accessible by integration")
		return
	}

	root := filepath.Join(w.dataDir, "repositories")
	handler := cgi.Handler{
		Path: w.gitBackend,
		Root: "/",
		Dir:  root,
		Env: []string{
			"GIT_PROJECT_ROOT=" + root,
			"GIT_HTTP_EXPORT_ALL=1",
		},
		InheritEnv: []string{"PATH"},
		Stderr:     os.Stderr,
	}
	canonicalRequest := request.Clone(request.Context())
	canonicalURL := *request.URL
	canonicalURL.Path = "/" + repository.owner + "/" + repository.name + ".git/" + strings.Join(parts[2:], "/")
	canonicalRequest.URL = new(canonicalURL)
	handler.ServeHTTP(response, canonicalRequest)
	if service == "git-receive-pack" {
		w.refreshPullSnapshots(repository)
	}
}

func gitService(request *http.Request, parts []string) (string, bool) {
	if len(parts) == 4 && parts[2] == "info" && parts[3] == "refs" && request.Method == http.MethodGet {
		service := request.URL.Query().Get("service")
		return service, service == "git-upload-pack" || service == "git-receive-pack"
	}
	if len(parts) == 3 && request.Method == http.MethodPost {
		return parts[2], parts[2] == "git-upload-pack" || parts[2] == "git-receive-pack"
	}
	return "", false
}

func resolveRef(gitDir, ref string) string {
	command := exec.Command("git", "--git-dir", gitDir, "rev-parse", "--verify", "refs/heads/"+ref+"^{commit}")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (w *world) refreshPullSnapshots(repository repository) {
	type refSnapshot struct {
		key     pullKey
		baseRef string
		headRef string
	}
	w.mu.RLock()
	snapshots := make([]refSnapshot, 0)
	for key, pull := range w.pulls {
		if pull.repositoryID == repository.id {
			snapshots = append(snapshots, refSnapshot{key: key, baseRef: pull.baseRef, headRef: pull.headRef})
		}
	}
	w.mu.RUnlock()

	for _, snapshot := range snapshots {
		baseSHA := resolveRef(repository.gitDir, snapshot.baseRef)
		headSHA := resolveRef(repository.gitDir, snapshot.headRef)
		w.mu.Lock()
		pull, found := w.pulls[snapshot.key]
		if found && pull.baseRef == snapshot.baseRef && pull.headRef == snapshot.headRef {
			if baseSHA != "" {
				pull.baseSHA = baseSHA
			}
			if headSHA != "" {
				pull.headSHA = headSHA
			}
			w.pulls[snapshot.key] = pull
		}
		w.mu.Unlock()
	}
}
