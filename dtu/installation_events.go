package dtu

import "fmt"

type installationEventAccount struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type installationEventInstallation struct {
	ID                  int64                    `json:"id"`
	Account             installationEventAccount `json:"account"`
	TargetType          string                   `json:"target_type"`
	RepositorySelection string                   `json:"repository_selection"`
	Permissions         map[string]string        `json:"permissions"`
	SuspendedAt         any                      `json:"suspended_at"`
}

type installationEventRepository struct {
	ID            int64                    `json:"id"`
	Name          string                   `json:"name"`
	FullName      string                   `json:"full_name"`
	Private       bool                     `json:"private"`
	DefaultBranch string                   `json:"default_branch"`
	HTMLURL       string                   `json:"html_url"`
	Owner         installationEventAccount `json:"owner"`
}

func (w *world) appendInstallationEvent(installation installation, action string) {
	payload := struct {
		Action       string                        `json:"action"`
		Installation installationEventInstallation `json:"installation"`
		Sender       eventSender                   `json:"sender"`
	}{
		Action:       action,
		Installation: installationEventValue(installation),
		Sender:       installationSender(installation),
	}
	w.appendPendingAppEvent("installation", action, installation.appID, 0, payload)
}

func (w *world) appendInstallationRepositoriesEvent(installation installation, repository repository, action string) {
	payload := struct {
		Action              string                        `json:"action"`
		Installation        installationEventInstallation `json:"installation"`
		RepositoriesAdded   []installationEventRepository `json:"repositories_added"`
		RepositoriesRemoved []installationEventRepository `json:"repositories_removed"`
		Sender              eventSender                   `json:"sender"`
	}{
		Action:              action,
		Installation:        installationEventValue(installation),
		RepositoriesAdded:   []installationEventRepository{installationRepositoryValue(installation, repository)},
		RepositoriesRemoved: []installationEventRepository{},
		Sender:              installationSender(installation),
	}
	w.appendPendingEvent("installation_repositories", action, repository.id, payload)
}

func installationEventValue(installation installation) installationEventInstallation {
	return installationEventInstallation{
		ID: installation.id,
		Account: installationEventAccount{
			Login: installation.owner,
			Type:  installation.ownerType,
		},
		TargetType:          installation.ownerType,
		RepositorySelection: "selected",
		Permissions:         copyMap(installation.permissions),
		SuspendedAt:         nil,
	}
}

func installationRepositoryValue(installation installation, repository repository) installationEventRepository {
	fullName := repository.owner + "/" + repository.name
	return installationEventRepository{
		ID:            repository.id,
		Name:          repository.name,
		FullName:      fullName,
		Private:       false,
		DefaultBranch: "main",
		HTMLURL:       "https://github.test/" + fullName,
		Owner: installationEventAccount{
			Login: installation.owner,
			Type:  installation.ownerType,
		},
	}
}

func installationSender(installation installation) eventSender {
	return eventSender{
		ID:    installation.appID,
		Login: fmt.Sprintf("dtu-app-%d[bot]", installation.appID),
		Type:  "Bot",
	}
}
