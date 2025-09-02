package githubclient

import (
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/genqlient"
	"github.com/stretchr/testify/require"
)

func TestMatchPullRequestStack(t *testing.T) {
	tests := []struct {
		name    string
		commits []git.Commit
		prs     genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection
		expect  []*github.PullRequest
	}{
		{
			name: "ThirdCommitQueue",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:      2,
						Id:              "20",
						HeadRefName:     "spr/master/00000002",
						BaseRefName:     "master",
						MergeQueueEntry: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestMergeQueueEntry{Id: "020"},
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1", MessageBody: "commit-id:1"},
								},
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2", MessageBody: "commit-id:2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
						Body:       "commit-id:2",
					},
					InQueue: true,
					Commits: []git.Commit{
						{CommitID: "1", CommitHash: "1", Body: "commit-id:1"},
						{CommitID: "2", CommitHash: "2", Body: "commit-id:2"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name: "FourthCommitQueue",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
				{CommitID: "00000004"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:      2,
						Id:              "20",
						HeadRefName:     "spr/master/00000002",
						BaseRefName:     "master",
						MergeQueueEntry: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestMergeQueueEntry{Id: "020"},
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1", MessageBody: "commit-id:1"},
								},
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2", MessageBody: "commit-id:2"},
								},
							},
						},
					},
					{
						DatabaseId:  3,
						Id:          "30",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "3", MessageBody: "commit-id:3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
						Body:       "commit-id:2",
					},
					InQueue: true,
					Commits: []git.Commit{
						{CommitID: "1", CommitHash: "1", Body: "commit-id:1"},
						{CommitID: "2", CommitHash: "2", Body: "commit-id:2"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "3",
					Id:         "30",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
						Body:       "commit-id:3",
					},
					Commits: []git.Commit{
						{CommitID: "3", CommitHash: "3", Body: "commit-id:3"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name:    "Empty",
			commits: []git.Commit{},
			prs:     genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{},
			expect:  []*github.PullRequest{},
		},
		{
			name:    "FirstCommit",
			commits: []git.Commit{{CommitID: "00000001"}},
			prs:     genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{},
			expect:  []*github.PullRequest{},
		},
		{
			name: "SecondCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "1",
					Id:         "10",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name: "ThirdCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
					{
						DatabaseId:  2,
						Id:          "20",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "1",
					Id:         "10",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name:    "RemoveOnlyCommit",
			commits: []git.Commit{},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{},
		},
		{
			name: "RemoveTopCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
					{
						DatabaseId:  3,
						Id:          "30",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2"},
								},
							},
						},
					},
					{
						DatabaseId:  2,
						Id:          "20",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "1",
					Id:         "10",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name: "RemoveMiddleCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000003"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
					{
						DatabaseId:  2,
						Id:          "20",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2"},
								},
							},
						},
					},
					{
						DatabaseId:  3,
						Id:          "30",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "1",
					Id:         "10",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "3",
					Id:         "30",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name: "RemoveBottomCommit",
			commits: []git.Commit{
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnection{
				Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
					{
						DatabaseId:  1,
						Id:          "10",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "1"},
								},
							},
						},
					},
					{
						DatabaseId:  2,
						Id:          "20",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "2"},
								},
							},
						},
					},
					{
						DatabaseId:  3,
						Id:          "30",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
							Nodes: []genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
								{
									genqlient.PullRequestsWithMergeQueueViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					DatabaseId: "1",
					Id:         "10",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},

				{
					DatabaseId: "2",
					Id:         "20",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					DatabaseId: "3",
					Id:         "30",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		repoConfig := &config.RepoConfig{}
		t.Run(tc.name, func(t *testing.T) {
			actual := matchPullRequestStack(repoConfig, "master", tc.commits, tc.prs)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestFormatPullRequestBody(t *testing.T) {
	simpleCommit := git.Commit{
		CommitID:   "abc123",
		CommitHash: "abcdef123456",
	}
	descriptiveCommit := git.Commit{
		CommitID:   "def456",
		CommitHash: "ghijkl7890",
		Body: `This body describes my nice PR.
It even includes some **markdown** formatting.`}

	tests := []struct {
		description string
		commit      git.Commit
		stack       []*github.PullRequest
	}{
		{
			description: "",
			commit:      git.Commit{},
			stack:       []*github.PullRequest{},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 2, Commit: descriptiveCommit},
			},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.

---

**Stack**:
- #2 ⬅
- #1


⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 1, Commit: simpleCommit},
				{Number: 2, Commit: descriptiveCommit},
			},
		},
	}

	for _, tc := range tests {
		body := FormatBody(tc.commit, tc.stack, false)
		if body != tc.description {
			t.Fatalf("expected: '%v', actual: '%v'", tc.description, body)
		}
	}
}

func TestFormatPullRequestBody_ShowPrTitle(t *testing.T) {
	simpleCommit := git.Commit{
		CommitID:   "abc123",
		CommitHash: "abcdef123456",
	}
	descriptiveCommit := git.Commit{
		CommitID:   "def456",
		CommitHash: "ghijkl7890",
		Body: `This body describes my nice PR.
It even includes some **markdown** formatting.`}

	tests := []struct {
		description string
		commit      git.Commit
		stack       []*github.PullRequest
	}{
		{
			description: "",
			commit:      git.Commit{},
			stack:       []*github.PullRequest{},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 2, Commit: descriptiveCommit},
			},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.

---

**Stack**:
- Title B #2 ⬅
- Title A #1


⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 1, Commit: simpleCommit, Title: "Title A"},
				{Number: 2, Commit: descriptiveCommit, Title: "Title B"},
			},
		},
	}

	for _, tc := range tests {
		body := FormatBody(tc.commit, tc.stack, true)
		if body != tc.description {
			t.Fatalf("expected: '%v', actual: '%v'", tc.description, body)
		}
	}
}

func TestInsertBodyIntoPRTemplateHappyPath(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "create PR",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: nil,
			expected: `
## Related Issues
<!--- Add any related issues here -->

## Description
inserted body

## Checklist
- [ ] My code follows the style guidelines of this project`,
		},
		{
			name: "update PR",
			body: "updated description",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: &github.PullRequest{
				Body: `
## Related Issues
* Issue #1234

## Description
original description

## Checklist
- [x] My code follows the style guidelines of this project`,
			},
			expected: `
## Related Issues
* Issue #1234

## Description
updated description

## Checklist
- [x] My code follows the style guidelines of this project`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := InsertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if body != tt.expected {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, body)
			}
		})
	}
}

func TestInsertBodyIntoPRTemplateErrors(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "no match insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "does not exist",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert start",
		},
		{
			name: "no match insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "does not exist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert end",
		},
		{
			name: "multiple many matches insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "duplicate",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert start",
		},
		{
			name: "multiple many matches insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "duplicate",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InsertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if !strings.Contains(err.Error(), tt.expected) {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, err.Error())
			}
		})
	}
}
