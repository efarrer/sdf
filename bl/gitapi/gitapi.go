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
	"github.com/go-git/go-git/v5/plumbing"
	gogithub "github.com/google/go-github/v69/github"
)

type GitApi struct {
	config *config.Config
	gitcmd git.GitInterface
	github github.GitHubInterface
}

func New(config *config.Config, gitcmd git.GitInterface, github github.GitHubInterface) GitApi {
	return GitApi{config: config, gitcmd: gitcmd, github: github}
}

// DeletePullRequest deletes the pull request and the associated branch
func (gapi GitApi) DeletePullRequest(ctx context.Context, pr *github.PullRequest) error {
	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName

	err := gapi.github.EditPullRequest2(ctx, owner, repoName, pr.Number, &gogithub.PullRequest{State: gogithub.Ptr("closed")})
	if err != nil {
		return fmt.Errorf("deleting pr %d %w", pr.Number, err)
	}

	return gapi.DeleteRemoteBranch(ctx, pr.FromBranch)
}

func (gapi GitApi) DeleteRemoteBranch(ctx context.Context, branch string) error {
	remoteName := gapi.config.Repo.GitHubRemote

	// Construct the reference name for branch
	refName := plumbing.NewBranchReferenceName(branch)

	// Delete the remote branch
	err := gapi.gitcmd.Push(remoteName, []string{fmt.Sprintf(":%s", refName)})
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

	destBranchRefName, err := gapi.gitcmd.OriginBranchRef(ctx, destBranchName)
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
	err = gitshell.Git(fmt.Sprintf("worktree add %s %s", tempDir, destBranchRefName), nil)
	if err != nil {
		return fmt.Errorf("creating the worktree in %s %w", tempDir, err)
	}
	cleanup.worktree = tempDir

	// Create a shell for the new worktree
	gitworktreeshell := realgit.NewGitCmd(gapi.config)
	gitworktreeshell.SetRootDir(tempDir)

	// Create the local branch if it doesn't already exist
	if branchExists, _ := gapi.gitcmd.BranchExists(branchName); !branchExists {
		err = gitworktreeshell.Git(fmt.Sprintf("checkout -b %s %s", branchName, destBranchRefName), nil)
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
		if strings.Contains(output, "Merge conflict in") || strings.HasPrefix(output, "CONFLICT ") {
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

func (gapi GitApi) CreatePullRequest(
	ctx context.Context,
	commit git.Commit,
	prevCommit *git.Commit,
) (*github.PullRequest, error) {

	headRefName, baseRefName := gapi.getBranches(commit, prevCommit)

	body, err := gapi.getBody(commit, nil)
	if err != nil {
		return nil, fmt.Errorf("getting body %w", err)
	}

	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName

	resp, err := gapi.github.CreatePullRequest2(ctx, owner, repoName, &gogithub.NewPullRequest{
		Title:    &commit.Subject,
		Head:     &headRefName,
		HeadRepo: &gapi.config.Repo.GitHubRepoName,
		Base:     &baseRefName,
		Body:     &body,
		Draft:    gogithub.Ptr(false), // We always create draft PRs then we do an update (to link them together) with an update.
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

// UpdatePullRequest - updates an existing PR.
// pullRequests is used to create links to related pull requests
// prevCommit is used to compute the destination branch name.
func (gapi GitApi) UpdatePullRequest(
	ctx context.Context,
	pullRequests []*github.PullRequest,
	pr *github.PullRequest,
	commit git.Commit,
	prevCommit *git.Commit,
) error {
	return gapi.updatePullRequest(ctx, pullRequests, pr, commit, prevCommit)
}

// UpdatePullRequestToMain - updates an existing PR to merge into main/master
// pullRequests is used to create links to related pull requests
func (gapi GitApi) UpdatePullRequestToMain(
	ctx context.Context,
	pullRequests []*github.PullRequest,
	pr *github.PullRequest,
	commit git.Commit,
) error {
	return gapi.updatePullRequest(ctx, pullRequests, pr, commit, nil)
}

// updatePullRequest - updates an existing PR.
// pullRequests is used to create links to related pull requests
// prevCommit is used to compute the destination branch name. If it is nil main/master will be used
func (gapi GitApi) updatePullRequest(
	ctx context.Context,
	pullRequests []*github.PullRequest,
	pr *github.PullRequest,
	commit git.Commit,
	prevCommit *git.Commit,
) error {

	// Note if prevCommit is nil then gapi.config.Repo.GitHubBranch is used
	headRefName, baseRefName := gapi.getBranches(commit, prevCommit)

	body, err := gapi.getBody(commit, pullRequests)
	if err != nil {
		return fmt.Errorf("getting body %w", err)
	}

	id, err := strconv.ParseInt(pr.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("converting ID %s to integer %w", pr.ID, err)
	}
	title := &commit.Subject
	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName

	head := gogithub.PullRequestBranch{
		Ref: gogithub.Ptr(headRefName),
	}
	base := gogithub.PullRequestBranch{
		Ref: gogithub.Ptr(baseRefName),
	}
	err = gapi.github.EditPullRequest2(ctx, owner, repoName, pr.Number, &gogithub.PullRequest{
		ID:    &id,
		Title: title,
		Body:  &body,
		Draft: &gapi.config.User.CreateDraftPRs,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return fmt.Errorf("updating PR for commit %s: %w", commit.CommitHash, err)
	}

	return nil
}

func (gapi GitApi) MergePullRequest(
	ctx context.Context,
	pr *github.PullRequest,
) error {
	owner := gapi.config.Repo.GitHubRepoOwner
	repoName := gapi.config.Repo.GitHubRepoName

	// Get the merge method
	mergeMethod := gapi.config.Repo.MergeMethod

	err := gapi.github.MergePullRequest2(ctx, owner, repoName, pr.Number, "", &gogithub.PullRequestOptions{
		MergeMethod: string(mergeMethod),
	})
	if err != nil {
		return fmt.Errorf("unable to merge %d %w", pr.Number, err)
	}

	return nil
}

// getBranches returns the head and base branch ref names
func (gapi GitApi) getBranches(commit git.Commit, prevCommit *git.Commit) (string, string) {
	baseRefName := gapi.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(gapi.config, *prevCommit)
	}
	headRefName := git.BranchNameFromCommit(gapi.config, commit)

	return headRefName, baseRefName
}

func (gapi GitApi) getBody(commit git.Commit, pullRequests []*github.PullRequest) (string, error) {
	body := githubclient.FormatBody(commit, pullRequests, gapi.config.Repo.ShowPrTitlesInStack)
	if gapi.config.Repo.PRTemplatePath == "" {
		return body, nil
	}

	fullTemplatePath := path.Join(gapi.gitcmd.RootDir(), gapi.config.Repo.PRTemplatePath)
	pullRequestTemplateBytes, err := os.ReadFile(fullTemplatePath)
	if err != nil {
		return "", fmt.Errorf("reading template file %s: %w", fullTemplatePath, err)
	}
	pullRequestTemplate := string(pullRequestTemplateBytes)

	body, err = githubclient.InsertBodyIntoPRTemplate(body, pullRequestTemplate, gapi.config.Repo, nil)
	if err != nil {
		return "", fmt.Errorf("inserting body into PR template %s: %w", fullTemplatePath, err)
	}

	return body, nil
}
