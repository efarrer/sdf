//go:build integration

package integration

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"testing"
	"time"

	"github.com/ejoffe/spr/bl"
	"github.com/ejoffe/spr/bl/gitapi"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/output/mockoutput"
	"github.com/ejoffe/spr/spr"
	ngit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	gogithub "github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/require"
)

// SacrificialRepo is an env var that must contain the path to the sacrificial repo for the integration tests to be
// executed.
const SacrificialRepo = "SACRIFICIAL_REPO"

// SacrificialRepo is a file that must exist in the repo or the tests won't run. This is an additional protection
// against running the tests against a "real" repo.
const SacrificialFile = "spr.sacrificial"

// prefix is a unique string that will be used to make git files and commit messages unique
var prefix string = ""

// resoruces contains various resources for unit testing
type resources struct {
	cfg        *config.Config
	goghclient *gogithub.Client
	gitshell   git.GitInterface
	stackedpr  *spr.Stackediff
	printer    *mockoutput.CapturedOutput
	commitIds  []string
	validate   func()
}

func initialize(t *testing.T, cfgfn func(*config.Config)) *resources {
	t.Helper()

	// Make sure we are working with a sacrificial repoPath
	repoPath := os.Getenv(SacrificialRepo)
	require.NotEqual(t, "", repoPath, fmt.Sprintf("must set the %s env var", SacrificialRepo))

	if !fileExists(path.Join(repoPath, ".git/config")) {
		require.Failf(t, "\"%s\" is not a git repo", SacrificialRepo)
	}
	if !fileExists(path.Join(repoPath, SacrificialFile)) {
		require.Failf(t, "\"%s\" is not marked as a sacrificial repo. Add and commit a file named \"%s\" to allow these integration tests to use it. Note this should not be done with any repo that contains valuable data.", SacrificialRepo, SacrificialFile)
	}
	err := os.Chdir(repoPath)
	require.NoError(t, err)

	// Create a unique identifier for this execution
	prefix = fmt.Sprintf("%d-", rand.Int())

	// Parse the config then overwrite the state and the global settings
	// This is so we can re-use the repos settings.
	gitcmd := realgit.NewGitCmd(config.DefaultConfig())
	//  check that we are inside a git dir
	var out string
	err = gitcmd.Git("status --porcelain", &out)
	require.NoError(t, err)

	cfg := config_parser.ParseConfig(gitcmd)
	// Overwrite State and User so the test has a consistent experience.
	cfgdefault := config.DefaultConfig()
	cfg.State = cfgdefault.State
	cfg.User = cfgdefault.User
	cfg.State.Stargazer = true
	cfgfn(cfg)

	err = config_parser.CheckConfig(cfg)
	require.NoError(t, err)

	gitcmd = realgit.NewGitCmd(cfg)

	goghclient := gogithub.NewClient(nil).WithAuthToken(github.FindToken(cfg.Repo.GitHubHost))

	ctx := context.Background()
	client := githubclient.NewGitHubClient(ctx, cfg)
	stackedpr := spr.NewStackedPR(cfg, client, gitcmd, goghclient)

	// Direct the output to a mock Printer so we can test against the output
	capout := mockoutput.MockPrinter()
	stackedpr.Printer = capout

	// Try and cleanup and reset the repo
	state, err := bl.NewReadState(ctx, cfg, goghclient, gitcmd)
	require.NoError(t, err)

	gitapi := gitapi.New(cfg, gitcmd, goghclient)
	for _, commit := range state.Commits {
		if commit.PullRequest != nil {
			gitapi.DeletePullRequest(ctx, commit.PullRequest)
		}
	}

	err = gitcmd.Git(fmt.Sprintf("reset --hard %s/%s", cfg.Repo.GitHubRemote, cfg.Repo.GitHubBranch), nil)
	require.NoError(t, err)

	r := &resources{
		cfg:        cfg,
		goghclient: goghclient,
		gitshell:   gitcmd,
		stackedpr:  stackedpr,
		printer:    capout,
	}

	// Add a function that will validate that all remote branches associated with any commits created by the unit test are
	// cleaned up
	r.validate = func() {
		branches, err := gitcmd.RemoteBranches()
		require.NoError(t, err)
		for _, commitId := range r.commitIds {
			branchName := fmt.Sprintf("refs/heads/%s", git.BranchNameFromCommitId(r.cfg, commitId))
			require.False(t, branches.Contains(branchName), fmt.Sprintf("%s should be deleted at the end of this integration test", branchName))
		}
	}

	return r
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// commit is the contents to add to a commit. If the filename exists the contents will be appended.
type commit struct {
	filename string
	contents string
}

// repo creates a *gogit.Repository the *gogit.Repository should not be shared between goroutines
func repo() *ngit.Repository {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	repo, err := ngit.PlainOpenWithOptions(cwd, &ngit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		fmt.Printf("%s is not a git repository\n", cwd)
		os.Exit(2)
	}

	return repo
}

// createCommits creates the commits
func (r *resources) createCommits(t *testing.T, commits []commit) {
	t.Helper()

	worktree, err := repo().Worktree()
	require.NoError(t, err)

	for _, commit := range commits {
		func() {
			file, err := os.OpenFile(commit.filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
			require.NoError(t, err)
			defer file.Close()

			_, err = file.WriteString(commit.contents)
			require.NoError(t, err)
		}()
		_, err = worktree.Add(commit.filename)
		require.NoError(t, err)

		commit, err := worktree.Commit(commit.contents, &ngit.CommitOptions{
			Author: &object.Signature{
				Name:  "Testy McTestFace",
				Email: "testy.mctestface@example.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		_, err = repo().CommitObject(commit)
		require.NoError(t, err)
	}

	// Capture the commit-ids for these commits so we can validate they got deleted
	ctx := context.Background()
	state, err := bl.NewReadState(ctx, r.cfg, r.goghclient, r.gitshell)
	require.NoError(t, err)
	for _, commit := range state.Commits {
		r.commitIds = append(r.commitIds, commit.CommitID)
	}

}

func TestBasicCommitUpdateMergeWithNoSubsetPRSets(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
		c.Repo.PRTemplatePath = "./PRTemplate.integration"
		c.Repo.PRTemplateInsertStart = "--START--"
		c.Repo.PRTemplateInsertEnd = "--END--"
	})
	defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			},
		})

		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s0.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PRs with spr merge", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.MergePRSet(ctx, "s0")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})
}

func TestBasicCommitUpdateMergeWithNoSubsetPRSetsInABranch(t *testing.T) {
	ctx := context.Background()
	remoteMain := ""
	gitHubBranch := ""
	resources := initialize(t, func(c *config.Config) {
		gitHubBranch = c.Repo.GitHubBranch
		remoteMain = c.Repo.GitHubRemote + "/" + c.Repo.GitHubBranch
	})
	defer resources.validate()
	name := prefix + t.Name()

	// Create a new branch.
	branchName := name
	err := resources.gitshell.Git(fmt.Sprintf("checkout -b %s %s", branchName, remoteMain), nil)
	require.NoError(t, err)

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s0.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PRs with spr merge", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.MergePRSet(ctx, "s0")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	// Clean up branch
	err = resources.gitshell.Git(fmt.Sprintf("checkout %s", gitHubBranch), nil)
	require.NoError(t, err)
	err = resources.gitshell.Git(fmt.Sprintf("branch -d %s", branchName), nil)
	require.NoError(t, err)
}

func TestBasicCommitUpdateMergeWithMultiplePRSets(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
	})
	defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			}, {
				filename: name + "3",
				contents: name + "3",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("3.*No Pull Request Created")
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PR sets with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-1")
		resources.stackedpr.UpdatePRSets(ctx, "2")
		resources.stackedpr.UpdatePRSets(ctx, "3")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("3.*s2.*github.com")
		resources.printer.ExpectRegExp("2.*s1.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PR sets with spr merge", func(t *testing.T) {
		resources.stackedpr.MergePRSet(ctx, "s2")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s1.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()

		resources.stackedpr.MergePRSet(ctx, "s1")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()

		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.MergePRSet(ctx, "s0")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})
}

func TestBasicCommitUpdateWithMergeConflictsWithSelectedCommits(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
	})
	defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.printer.ExpectString("no local commits\n")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			}, {
				filename: name + "0",
				contents: "more content" + name + "0",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("3.*No Pull Request Created")
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Try to create PRs but get merge conflict due to skipping a dependent commit", func(t *testing.T) {
		require.Panicsf(t, func() {
			os.Setenv("SPR_DEBUG", "1") // Hack to force a panic instead of os.Exit(1)
			resources.stackedpr.UpdatePRSets(ctx, "1-3")
		}, "Expected a panic when a commit is includes that can't be cherry picked onto the existing commits")
	})
}

func TestBasicCommitUpdateReOrderCommitsReUpdateMerge(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
	})
	defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s0.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	// Reorder commits
	t.Run("Reorder commits", func(t *testing.T) {
		// First get commit sha1s
		var output string
		state, err := bl.NewReadState(ctx, resources.cfg, resources.goghclient, resources.gitshell)
		require.NoError(t, err)

		// Then reset hard
		err = resources.gitshell.Git(fmt.Sprintf("reset --hard %s/%s", resources.cfg.Repo.GitHubRemote, resources.cfg.Repo.GitHubBranch), &output)
		require.NoError(t, err)

		// Now cherry-pick commits out of order
		for _, commit := range state.Commits {
			err = resources.gitshell.Git(fmt.Sprintf("cherry-pick %s", commit.CommitHash), &output)
			require.NoError(t, err)
		}
	})

	t.Run("Can update PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s1.*github.com")
		resources.printer.ExpectRegExp("1.*s1.*github.com")
		resources.printer.ExpectRegExp("0.*s1.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PRs with spr merge", func(t *testing.T) {
		resources.stackedpr.MergePRSet(ctx, "s1")
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})
}

func TestBasicCommitUpdateRemoveCommitReUpdateMerge(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
	})
	//defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s0.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	// Remove a commit
	t.Run("Remove a commit", func(t *testing.T) {
		// First get commit sha1s
		var output string
		state, err := bl.NewReadState(ctx, resources.cfg, resources.goghclient, resources.gitshell)
		require.NoError(t, err)

		// Then reset hard
		err = resources.gitshell.Git(fmt.Sprintf("reset --hard %s/%s", resources.cfg.Repo.GitHubRemote, resources.cfg.Repo.GitHubBranch), &output)
		require.NoError(t, err)

		// Now cherry-pick only the first and last commits (not the middle)
		err = resources.gitshell.Git(fmt.Sprintf("cherry-pick %s", state.Commits[2].CommitHash), &output)
		require.NoError(t, err)
		err = resources.gitshell.Git(fmt.Sprintf("cherry-pick %s", state.Commits[0].CommitHash), &output)
		require.NoError(t, err)
	})

	t.Run("Can update PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "s0")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("1.*s1.*github.com")
		resources.printer.ExpectRegExp("0.*s1.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PRs with spr merge", func(t *testing.T) {
		resources.stackedpr.MergePRSet(ctx, "s1")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)

		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})
}

func TestBasicCommitUpdateMergeWithMergeCheck(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
		c.Repo.MergeCheck = "/bin/ls"
	})
	defer resources.validate()
	name := prefix + t.Name()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})

	t.Run("New commits are shown with spr status", func(t *testing.T) {
		resources.createCommits(t, []commit{
			{
				filename: name + "0",
				contents: name + "0",
			}, {
				filename: name + "1",
				contents: name + "1",
			}, {
				filename: name + "2",
				contents: name + "2",
			},
		})

		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*No Pull Request Created")
		resources.printer.ExpectRegExp("1.*No Pull Request Created")
		resources.printer.ExpectRegExp("0.*No Pull Request Created")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can create PRs with spr update", func(t *testing.T) {
		resources.stackedpr.UpdatePRSets(ctx, "0-2")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectString(spr.Header(resources.cfg))
		resources.printer.ExpectRegExp("2.*s0.*github.com")
		resources.printer.ExpectRegExp("1.*s0.*github.com")
		resources.printer.ExpectRegExp("0.*s0.*github.com")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can't merge without spr check first", func(t *testing.T) {
		require.Panicsf(t, func() {
			os.Setenv("SPR_DEBUG", "1") // Hack to force a panic instead of os.Exit(1)
			resources.stackedpr.MergePRSet(ctx, "s0")
		}, "Expected a panic when a spr check is needed but hasn't been executed")
	})

	t.Run("Run merge check", func(t *testing.T) {
		resources.stackedpr.RunMergeCheck(ctx)
	})

	t.Run("Can merge after spr check", func(t *testing.T) {
		resources.stackedpr.MergePRSet(ctx, "s0")

		resources.printer.Purge()
		resources.stackedpr.StatusCommitsAndPRSets(ctx)

		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})
}

func TestMergeWithInvalidPRSetFails(t *testing.T) {
	ctx := context.Background()
	resources := initialize(t, func(c *config.Config) {
	})
	defer resources.validate()

	t.Run("Starts in expected state", func(t *testing.T) {
		resources.stackedpr.StatusCommitsAndPRSets(ctx)
		resources.printer.ExpectRegExp(".*no local commits.*")
		resources.printer.ExpectationsMet()
	})

	t.Run("Can merge PRs with spr merge", func(t *testing.T) {
		require.Panicsf(t, func() {
			os.Setenv("SPR_DEBUG", "1") // Hack to force a panic instead of os.Exit(1)
			resources.stackedpr.MergePRSet(ctx, "s0")
		}, "Expected a panic when a spr merge with an invalid PR set")
		resources.printer.ExpectationsMet()
	})
}
