package spr

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/ejoffe/spr/github/mockclient"
	"github.com/ejoffe/spr/mock"
	"github.com/ejoffe/spr/output"
)

func makeTestObjects(t *testing.T, synchronized bool) (
	s *Stackediff, gitmock *mockgit.Mock, githubmock *mockclient.MockClient,
	input *bytes.Buffer, capout *output.CapturedOutput) {
	cfg := config.EmptyConfig()
	cfg.Repo.RequireChecks = true
	cfg.Repo.RequireApproval = true
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "master"
	cfg.Repo.MergeMethod = "rebase"
	expectations := mock.New(t, synchronized)
	gitmock = mockgit.NewMockGit(expectations)
	githubmock = mockclient.NewMockClient(expectations)
	githubmock.Info = &github.GitHubInfo{
		UserName:     "TestSPR",
		RepositoryID: "RepoID",
		LocalBranch:  "master",
	}
	s = NewStackedPR(cfg, githubmock, gitmock, nil, nil)
	capout = output.MockPrinter()
	s.Printer = capout

	input = &bytes.Buffer{}
	s.input = input
	s.synchronized = synchronized
	githubmock.Synchronized = synchronized
	return
}

func TestSPRBasicFlowFourCommitsQueue(t *testing.T) {
	testSPRBasicFlowFourCommitsQueue(t, true)
	testSPRBasicFlowFourCommitsQueue(t, false)
}

func testSPRBasicFlowFourCommitsQueue(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		capout.ExpectString("pull request stack is empty\n")
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		pr := github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(Header(s.config))
		capout.ExpectString(pr.Stringer(s.config))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n")
		capout.ExpectString(Header(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 2",
		}
		capout.ExpectString(pr.Stringer(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c3, &c4})

		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		// For the first "create" call we should *not* call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString(Header(s.config))
		for i := 4; i > 0; i-- {
			pr = github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch")
		count := uint(2)
		s.MergePullRequests(ctx, &count)
		capout.ExpectString("MERGED https://///pull/1     : test commit 1")
		capout.ExpectString("MERGED https://///pull/1     : test commit 2")
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		gitmock.ExpectFetch()
		githubmock.Info.PullRequests = githubmock.Info.PullRequests[1:]
		githubmock.Info.PullRequests[0].Merged = false
		githubmock.Info.PullRequests[0].Commits = append(githubmock.Info.PullRequests[0].Commits, c1, c2)
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectStatus()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()

		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString(Header(s.config))
		for i := 4; i > 1; i-- {
			commits := []git.Commit{}
			if i == 2 {
				commits = []git.Commit{{}, {}}
			}

			pr = github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Commits: commits,
				Title:   fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()

		// 'git spr merge' :: MergePullRequest :: commits=[a2, a3, a4]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c4, nil)
		githubmock.ExpectMergePullRequest(c4, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch")

		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)
		gitmock.ExpectDeleteBranch("from_branch")

		githubmock.Info.PullRequests[0].InQueue = true

		s.MergePullRequests(ctx, nil)
		capout.ExpectString("MERGED ⌛ https://///pull/1     : test commit 2").
			ExpectString("MERGED https://///pull/1     : test commit 3").
			ExpectString("MERGED https://///pull/1     : test commit 4").
			ExpectationsMet()

		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()
	})
}

func TestSPRBasicFlowFourCommits(t *testing.T) {
	testSPRBasicFlowFourCommits(t, true)
	testSPRBasicFlowFourCommits(t, false)
}

func testSPRBasicFlowFourCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		capout.ExpectString("pull request stack is empty\n")
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString(Header(s.config))
		pr := github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n")
		capout.ExpectString(Header(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 2",
		}
		capout.ExpectString(pr.Stringer(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c3, &c4})

		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		// For the first "create" call we should *not* call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString("warning: not updating reviewers for PR #1\n").
			ExpectString(Header(s.config))
		for i := 4; i > 0; i-- {
			pr = github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2, a3, a4]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c4, nil)
		githubmock.ExpectMergePullRequest(c4, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)
		gitmock.ExpectDeleteBranch("from_branch")
		s.MergePullRequests(ctx, nil)
		for i := 1; i < 5; i++ {
			pr = github.PullRequest{
				Merged: true,
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
	})
}

func TestSPRBasicFlowDeleteBranch(t *testing.T) {
	testSPRBasicFlowDeleteBranch(t, true)
	testSPRBasicFlowDeleteBranch(t, false)
}

func testSPRBasicFlowDeleteBranch(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString(Header(s.config))
		pr := github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		capout.ExpectString("warning: not updating reviewers for PR #1\n")
		capout.ExpectString(Header(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 2",
		}
		capout.ExpectString(pr.Stringer(s.config))
		pr = github.PullRequest{
			Number: 1,
			MergeStatus: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
				Stacked:        true,
			},
			Title: "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch") // <--- This is the key expectation of this test.
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch") // <--- This is the key expectation of this test.
		s.MergePullRequests(ctx, nil)
		pr = github.PullRequest{
			Number: 1,
			Merged: true,
			Title:  "test commit 1",
		}
		capout.ExpectString(pr.Stringer(s.config))
		pr = github.PullRequest{
			Number: 1,
			Merged: true,
			Title:  "test commit 2",
		}
		capout.ExpectString(pr.Stringer(s.config))
		capout.ExpectationsMet()
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
	})
}

func TestSPRMergeCount(t *testing.T) {
	testSPRMergeCount(t, true)
	testSPRMergeCount(t, false)
}

func testSPRMergeCount(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for i := 4; i > 0; i-- {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr merge --count 2' :: MergePullRequest :: commits=[a1, a2, a3, a4]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch")
		for i := 1; i <= 2; i++ {
			pr := github.PullRequest{
				Merged: true,
				Number: 1,
				Title:  fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.MergePullRequests(ctx, uintptr(2))
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()
	})
}

func TestSPRAmendCommit(t *testing.T) {
	testSPRAmendCommit(t, true)
	testSPRAmendCommit(t, false)
}

func testSPRAmendCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}

		// 'git spr state' :: StatusPullRequest
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		capout.ExpectString("pull request stack is empty\n")
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// amend commit c2
		c2.CommitHash = "c201000000000000000000000000000000000000"
		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// amend commit c1
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c2.CommitHash = "c202000000000000000000000000000000000000"
		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch")
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch")
		for _, i := range []int{1, 2} {
			pr := github.PullRequest{
				Merged: true,
				Number: 1,
				Title:  fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.MergePullRequests(ctx, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()
	})
}

func TestSPRReorderCommit(t *testing.T) {
	testSPRReorderCommit(t, true)
	testSPRReorderCommit(t, false)
}

func testSPRReorderCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}
		c5 := git.Commit{
			CommitID:   "00000005",
			CommitHash: "c500000000000000000000000000000000000000",
			Subject:    "test commit 5",
		}

		// 'git spr status' :: StatusPullRequest
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		capout.ExpectString("pull request stack is empty\n")
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{4, 3, 2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c3, &c1, &c4, &c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, nil)
		githubmock.ExpectUpdatePullRequest(c4, nil)
		// reorder commits
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c2.CommitHash = "c201000000000000000000000000000000000000"
		c3.CommitHash = "c301000000000000000000000000000000000000"
		c4.CommitHash = "c401000000000000000000000000000000000000"
		gitmock.ExpectPushCommits([]*git.Commit{&c2, &c4, &c1, &c3})
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c2)
		githubmock.ExpectUpdatePullRequest(c1, &c4)
		githubmock.ExpectUpdatePullRequest(c3, &c1)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{4, 3, 2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c5, c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1, &c2, &c3, &c4, &c5})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, nil)
		githubmock.ExpectUpdatePullRequest(c4, nil)
		// reorder commits
		c1.CommitHash = "c102000000000000000000000000000000000000"
		c2.CommitHash = "c202000000000000000000000000000000000000"
		c3.CommitHash = "c302000000000000000000000000000000000000"
		c4.CommitHash = "c402000000000000000000000000000000000000"
		gitmock.ExpectPushCommits([]*git.Commit{&c5, &c4, &c3, &c2, &c1})
		githubmock.ExpectCreatePullRequest(c5, nil)
		githubmock.ExpectUpdatePullRequest(c5, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c5)
		githubmock.ExpectUpdatePullRequest(c3, &c4)
		githubmock.ExpectUpdatePullRequest(c2, &c3)
		githubmock.ExpectUpdatePullRequest(c1, &c2)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{5, 4, 3, 2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// TODO : add a call to merge and check merge order
	})
}

func TestSPRDeleteCommit(t *testing.T) {
	testSPRDeleteCommit(t, true)
	testSPRDeleteCommit(t, false)
}

func testSPRDeleteCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		capout.ExpectString("pull request stack is empty\n")
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{4, 3, 2, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}

		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// 'git spr update' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
		gitmock.ExpectFetch()
		githubmock.ExpectGetInfo()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c1})
		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)
		// update commits
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c4.CommitHash = "c401000000000000000000000000000000000000"
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c4})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c1)
		githubmock.ExpectGetInfo()
		capout.ExpectString(Header(s.config))
		for _, i := range []int{4, 1} {
			pr := github.PullRequest{
				Number: 1,
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
					Stacked:        true,
				},
				Title: fmt.Sprintf("test commit %d", i),
			}
			capout.ExpectString(pr.Stringer(s.config))
		}
		s.UpdatePullRequests(ctx, nil, nil)
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		capout.ExpectationsMet()

		// TODO : add a call to merge and check merge order
	})
}

func TestAmendNoCommits(t *testing.T) {
	testAmendNoCommits(t, true)
	testAmendNoCommits(t, false)
}

func testAmendNoCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, _, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		gitmock.ExpectLogAndRespond([]*git.Commit{})
		s.AmendCommit(ctx)
		capout.ExpectString("No commits to amend\n")
		capout.ExpectationsMet()
	})
}

func TestAmendOneCommit(t *testing.T) {
	testAmendOneCommit(t, true)
	testAmendOneCommit(t, false)
}

func testAmendOneCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectFixup(c1.CommitHash)
		input.WriteString("1")
		s.AmendCommit(ctx)
		capout.ExpectString(" 1 : 00000001 : test commit 1\n").
			ExpectString("Commit to amend (1): ")
		capout.ExpectationsMet()
	})
}

func TestAmendTwoCommits(t *testing.T) {
	testAmendTwoCommits(t, true)
	testAmendTwoCommits(t, false)
}

func testAmendTwoCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1, &c2})
		gitmock.ExpectFixup(c2.CommitHash)
		input.WriteString("1")
		s.AmendCommit(ctx)
		capout.ExpectString(" 2 : 00000001 : test commit 1\n").
			ExpectString(" 1 : 00000002 : test commit 2\n").
			ExpectString("Commit to amend (1-2): ")
		capout.ExpectationsMet()
	})
}

func TestAmendInvalidInput(t *testing.T) {
	testAmendInvalidInput(t, true)
	testAmendInvalidInput(t, false)
}

func testAmendInvalidInput(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, capout := makeTestObjects(t, sync)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("a")
		s.AmendCommit(ctx)
		capout.ExpectString(" 1 : 00000001 : test commit 1\n").
			ExpectString("Commit to amend (1): ").
			ExpectString("InvalidInput\n")
		gitmock.ExpectationsMet()
		capout.ExpectationsMet()

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("0")
		s.AmendCommit(ctx)
		capout.ExpectString(" 1 : 00000001 : test commit 1\n").
			ExpectString("Commit to amend (1): ").
			ExpectString("InvalidInput\n")
		gitmock.ExpectationsMet()
		capout.ExpectationsMet()

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("2")
		s.AmendCommit(ctx)
		capout.ExpectString(" 1 : 00000001 : test commit 1\n").
			ExpectString("Commit to amend (1): ").
			ExpectString("InvalidInput\n")
		gitmock.ExpectationsMet()
		capout.ExpectationsMet()
	})
}

func uintptr(a uint) *uint {
	return &a
}
