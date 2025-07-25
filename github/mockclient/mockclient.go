package mockclient

import (
	"context"
	"fmt"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/ejoffe/spr/mock"
)

const (
	NobodyUserID = "U_kgDOBb2UmA"
	NobodyLogin  = "nobody"
)

// NewMockClient creates a new mock client
func NewMockClient(expectations *mock.Expectations) *MockClient {
	return &MockClient{
		expectations: expectations,
	}
}

type MockClient struct {
	Info         *github.GitHubInfo
	expectations *mock.Expectations
	Synchronized bool // When true code is executed without goroutines. Allows test to be deterministic
}

func (c *MockClient) GetInfo(ctx context.Context, gitcmd git.GitInterface) *github.GitHubInfo {
	fmt.Printf("HUB: GetInfo\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op: mock.GetInfoOP,
	})
	return c.Info
}

func (c *MockClient) GetAssignableUsers(ctx context.Context) []github.RepoAssignee {
	fmt.Printf("HUB: GetAssignableUsers\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op: mock.GetAssignableUsersOP,
	})
	return []github.RepoAssignee{
		{
			ID:    NobodyUserID,
			Login: NobodyLogin,
			Name:  "No Body",
		},
	}
}

func (c *MockClient) CreatePullRequest(ctx context.Context, gitcmd git.GitInterface, info *github.GitHubInfo,
	commit git.Commit, prevCommit *git.Commit) *github.PullRequest {
	fmt.Printf("HUB: CreatePullRequest\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:     mock.CreatePullRequestOP,
		Commit: commit,
		Prev:   prevCommit,
	})

	// TODO - don't hardcode ID and Number
	// TODO - set FromBranch and ToBranch correctly
	return &github.PullRequest{
		ID:         "001",
		Number:     1,
		FromBranch: "from_branch",
		ToBranch:   "to_branch",
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: github.PullRequestMergeStatus{
			ChecksPass:     github.CheckStatusPass,
			ReviewApproved: true,
			NoConflicts:    true,
			Stacked:        true,
		},
	}
}

func (c *MockClient) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface, pullRequests []*github.PullRequest, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {
	fmt.Printf("HUB: UpdatePullRequest\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:     mock.UpdatePullRequestOP,
		Commit: commit,
		Prev:   prevCommit,
	})
}

func (c *MockClient) AddReviewers(ctx context.Context, pr *github.PullRequest, userIDs []string) {
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:      mock.AddReviewersOP,
		UserIDs: userIDs,
	})
}

func (c *MockClient) CommentPullRequest(ctx context.Context, pr *github.PullRequest, comment string) {
	fmt.Printf("HUB: CommentPullRequest\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:     mock.CommentPullRequestOP,
		Commit: pr.Commit,
	})
}

func (c *MockClient) MergePullRequest(ctx context.Context,
	pr *github.PullRequest, mergeMethod genclient.PullRequestMergeMethod) {
	fmt.Printf("HUB: MergePullRequest, method=%q\n", mergeMethod)
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:          mock.MergePullRequestOP,
		Commit:      pr.Commit,
		MergeMethod: mergeMethod,
	})
}

func (c *MockClient) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
	fmt.Printf("HUB: ClosePullRequest\n")
	c.expectations.GithubApi(mock.GithubExpectation{
		Op:     mock.ClosePullRequestOP,
		Commit: pr.Commit,
	})
}

func (c *MockClient) GetClient() genclient.Client {
	// This client can't be used it is just to satisfy the interface
	return genclient.NewClient("", nil)
}

func (c *MockClient) ExpectGetInfo() {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op: mock.GetInfoOP,
	})
}

func (c *MockClient) ExpectGetAssignableUsers() {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op: mock.GetAssignableUsersOP,
	})
}

func (c *MockClient) ExpectCreatePullRequest(commit git.Commit, prev *git.Commit) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:     mock.CreatePullRequestOP,
		Commit: commit,
		Prev:   prev,
	})
}

func (c *MockClient) ExpectUpdatePullRequest(commit git.Commit, prev *git.Commit) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:     mock.UpdatePullRequestOP,
		Commit: commit,
		Prev:   prev,
	})
}

func (c *MockClient) ExpectAddReviewers(userIDs []string) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:      mock.AddReviewersOP,
		UserIDs: userIDs,
	})
}

func (c *MockClient) ExpectCommentPullRequest(commit git.Commit) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:     mock.CommentPullRequestOP,
		Commit: commit,
	})
}

func (c *MockClient) ExpectMergePullRequest(commit git.Commit, mergeMethod genclient.PullRequestMergeMethod) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:          mock.MergePullRequestOP,
		Commit:      commit,
		MergeMethod: mergeMethod,
	})
}

func (c *MockClient) ExpectClosePullRequest(commit git.Commit) {
	c.expectations.ExpectGitHub(mock.GithubExpectation{
		Op:     mock.ClosePullRequestOP,
		Commit: commit,
	})
}

func (c *MockClient) ExpectationsMet() {
	c.expectations.ExpectationsMet()
}
