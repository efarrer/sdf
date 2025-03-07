package gitapi

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient"
	ngit "github.com/go-git/go-git/v5"
	ngitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gogithub "github.com/google/go-github/v69/github"
)

type GitApi struct {
	config     *config.Config
	repo       *ngit.Repository
	goghclient *gogithub.Client
}

func New(config *config.Config, repo *ngit.Repository, goghclient *gogithub.Client) GitApi {
	return GitApi{config: config, repo: repo, goghclient: goghclient}
}

// OriginMainRef returns the ref for the default remote and the default branch (often origin/main)
func (gapi GitApi) OriginMainRef(ctx context.Context) (*plumbing.Reference, error) {
	branch := gapi.config.Repo.GitHubBranch

	return gapi.OriginBranchRef(ctx, branch)
}

// OriginBranchRef returns the ref for the default remote (often origin) and the given branch
func (gapi GitApi) OriginBranchRef(ctx context.Context, branch string) (*plumbing.Reference, error) {
	remote := gapi.config.Repo.GitHubRemote

	originMainRef, err := gapi.repo.Reference(plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/%s", remote, branch)), true)
	if err != nil {
		return nil, fmt.Errorf("getting %s/%s HEAD %w", remote, branch, err)
	}

	return originMainRef, nil
}

// DeletePullRequest deletes the pull request and the associated branch
func (gapi GitApi) DeletePullRequest(ctx context.Context, pr *github.PullRequest) error {
	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName

	_, _, err := gapi.goghclient.PullRequests.Edit(ctx, owner, repoName, pr.Number, &gogithub.PullRequest{State: gogithub.Ptr("closed")})
	if err != nil {
		return fmt.Errorf("deleting pr %d %w", pr.Number, err)
	}

	return gapi.DeleteRemoteBranch(ctx, pr.FromBranch)
}

func (gapi GitApi) DeleteRemoteBranch(ctx context.Context, branch string) error {
	remoteName := gapi.config.Repo.GitHubRemote

	remote, err := gapi.repo.Remote(remoteName)
	if err != nil {
		return fmt.Errorf("getting remote %s %w", remoteName, err)
	}

	// Construct the reference name for branch
	refName := plumbing.NewBranchReferenceName(branch)

	pushOptions := ngit.PushOptions{
		RemoteName: remoteName,
		// Nothing before the colon says to push nothing to the destination branch (which deletes it).
		RefSpecs: []ngitconfig.RefSpec{ngitconfig.RefSpec(fmt.Sprintf(":%s", refName))},
	}

	// Delete the remote branch
	err = remote.Push(&pushOptions)
	if err != nil {
		return fmt.Errorf("removing remote branch %s %w", branch, err)
	}

	return nil
}

// CreateRemoteBranchWithCherryPick creates the remote branch `branchname` on `destBranchRef` and cherry-picks the sha
// on it. Returns a reference to the new branch.
func (gapi GitApi) CreateRemoteBranchWithCherryPick(ctx context.Context, branchName string, destBranchName string, sha string) error {
	// The "github.com/go-git/go-git/" doesn't support cherry picks so we
	//have to do this by shelling out to the command line
	gitshell := realgit.NewGitCmd(gapi.config)

	destBranchRef, err := gapi.OriginBranchRef(ctx, destBranchName)
	if err != nil {
		return fmt.Errorf("getting the ref for %s %w", destBranchName, err)
	}

	// cleanup code
	cleanup := struct {
		dir      string
		worktree string
		branch   string
	}{}
	defer func() {
		if cleanup.worktree != "" {
			gitshell.Git(fmt.Sprintf("worktree remove --force %s", cleanup.worktree), nil)
			gitshell.Git(fmt.Sprintf("worktree prune"), nil)
		}

		if cleanup.branch != "" {
			gitshell.Git(fmt.Sprintf("branch -D %s", branchName), nil)
		}

		if cleanup.dir != "" {
			os.RemoveAll(cleanup.dir)
		}
	}()

	// Create a temp dir for a new worktree
	tempDir, err := os.MkdirTemp("", strings.ReplaceAll(branchName, "/", "-"))
	if err != nil {
		return fmt.Errorf("creating the temp dir %w", err)
	}
	cleanup.dir = tempDir

	// Create the worktree
	err = gitshell.Git(fmt.Sprintf("worktree add %s %s", tempDir, destBranchRef.Hash().String()), nil)
	if err != nil {
		return fmt.Errorf("creating the worktree in %s %w", tempDir, err)
	}
	cleanup.worktree = tempDir

	// Create a shell for the new worktree
	gitworktreeshell := realgit.NewGitCmd(gapi.config)
	gitworktreeshell.SetRootDir(tempDir)

	// Create the local branch if it doesn't already exist
	if branchExists, _ := gapi.BranchExists(branchName); !branchExists {
		err = gitworktreeshell.Git(fmt.Sprintf("checkout -b %s %s", branchName, destBranchRef.Hash().String()), nil)
		if err != nil {
			return fmt.Errorf("creating the branch %s in worktree %s %w", branchName, tempDir, err)
		}
	}
	cleanup.branch = branchName

	// Cherry pick commit over to this branch.
	// Output a meaningful error message if we can't apply the cherry-pick
	output := ""
	gitworktreeshell.SetStderr(io.Discard)
	err = gitworktreeshell.Git(fmt.Sprintf("cherry-pick %s", sha), &output)
	if err != nil {
		if strings.Contains(output, "Merge conflict in") {
			return fmt.Errorf("Unable to add %s to the PR set as an earlier commit is required for it to merge properly.\n", sha)
		}
		return fmt.Errorf("cherry picking %s into %s in worktree %s %w", sha, branchName, tempDir, err)
	}

	// Push the branch up to the remote
	err = gitworktreeshell.Git(fmt.Sprintf("push --force %s %s:%s", gapi.config.Repo.GitHubRemote, branchName, branchName), nil)
	if err != nil {
		return fmt.Errorf("pushing %s to %s %w", branchName, gapi.config.Repo.GitHubRemote, err)
	}

	return nil
}

func (gapi GitApi) AppendCommitId() error {
	// The "github.com/go-git/go-git/" doesn't (easily) support updating a commit message so we have to do this by
	// shelling out to the command line
	gitshell := realgit.NewGitCmd(gapi.config)

	rewordPath, err := exec.LookPath("spr_reword_helper")
	if err != nil {
		fmt.Errorf("can't find spr_reword_helper %w", err)
	}
	rebaseCommand := fmt.Sprintf(
		"rebase %s/%s -i --autosquash --autostash",
		gapi.config.Repo.GitHubRemote,
		gapi.config.Repo.GitHubBranch,
	)
	err = gitshell.GitWithEditor(rebaseCommand, nil, rewordPath)
	if err != nil {
		fmt.Errorf("can't execute spr_reword_helper %w", err)
	}

	return nil
}

func (gapi GitApi) BranchExists(branchName string) (bool, error) {
	iter, err := gapi.repo.Branches()
	if err != nil {
		return false, fmt.Errorf("finding existing branches %w", err)
	}
	defer iter.Close()

	branchExists := false
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().String() == fmt.Sprintf("refs/heads/%s", branchName) {
			branchExists = true
		}
		if ref.Name().String() == branchName {
			branchExists = true
		}
		return nil
	})
	return branchExists, nil
}

func (gapi GitApi) CreatePullRequest(
	ctx context.Context,
	commit git.Commit,
	prevCommit *git.Commit,
) (*github.PullRequest, error) {

	baseRefName := gapi.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(gapi.config, *prevCommit)
	}
	headRefName := git.BranchNameFromCommit(gapi.config, commit)

	body := githubclient.FormatBody(commit, nil, gapi.config.Repo.ShowPrTitlesInStack)
	if gapi.config.Repo.PRTemplatePath != "" {
		w, err := gapi.repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("getting worktree %w", err)
		}
		fullTemplatePath := path.Join(w.Filesystem.Root(), gapi.config.Repo.PRTemplatePath)
		pullRequestTemplateBytes, err := os.ReadFile(fullTemplatePath)
		if err != nil {
			return nil, fmt.Errorf("reading template file %s: %w", fullTemplatePath, err)
		}
		pullRequestTemplate := string(pullRequestTemplateBytes)

		body, err = githubclient.InsertBodyIntoPRTemplate(body, pullRequestTemplate, gapi.config.Repo, nil)
		if err != nil {
			return nil, fmt.Errorf("inserting body into PR template %s: %w", fullTemplatePath, err)
		}
	}

	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName
	resp, _, err := gapi.goghclient.PullRequests.Create(ctx, owner, repoName, &gogithub.NewPullRequest{
		Title:    &commit.Subject,
		Head:     &headRefName,
		HeadRepo: &gapi.config.Repo.GitHubRepoName,
		Base:     &baseRefName,
		Body:     &body,
		Draft:    &gapi.config.User.CreateDraftPRs,
	})
	if err != nil {
		return nil, fmt.Errorf("creating PR for commit %s: %w", commit.CommitHash, err)
	}

	pr := &github.PullRequest{
		ID:         strconv.FormatInt(*resp.ID, 10),
		Number:     *resp.Number,
		FromBranch: headRefName,
		ToBranch:   baseRefName,
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: github.PullRequestMergeStatus{
			ChecksPass:     github.CheckStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}

	return pr, nil
}
