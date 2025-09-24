package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyConfig(t *testing.T) {
	expect := &Config{
		Repo: &RepoConfig{},
		User: &UserConfig{},
		State: &InternalState{
			MergeCheckCommit:      map[string]string{},
			RepoToCommitIdToPRSet: map[string]map[string]int{},
		},
	}
	actual := EmptyConfig()
	assert.Equal(t, expect, actual)
}

func TestDefaultConfig(t *testing.T) {
	expect := &Config{
		Repo: &RepoConfig{
			GitHubRepoOwner:       "",
			GitHubRepoName:        "",
			GitHubRemote:          "origin",
			GitHubBranch:          "main",
			GitHubHost:            "github.com",
			RequireChecks:         true,
			RequireApproval:       true,
			MergeMethod:           "rebase",
			PRTemplatePath:        "",
			PRTemplateInsertStart: "",
			PRTemplateInsertEnd:   "",
			ShowPrTitlesInStack:   false,
		},
		User: &UserConfig{
			LogGitCommands: false,
			LogGitHubCalls: false,
		},
		State: &InternalState{
			MergeCheckCommit:      map[string]string{},
			RepoToCommitIdToPRSet: map[string]map[string]int{},
		},
	}
	actual := DefaultConfig()
	assert.Equal(t, expect, actual)
}
