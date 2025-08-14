package internal_test

import (
	"context"
	"testing"

	"github.com/ejoffe/spr/bl/internal"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/fezzik_types"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestNewStateQL(t *testing.T) {
	ctx := context.Background()
	config := config.EmptyConfig()
	prAndStatus := &genclient.PullRequestsAndStatusResponse{}
	commits := []*object.Commit{}

	state, err := internal.NewStateQL(ctx, config, prAndStatus, commits)
	require.NoError(t, err)
	require.NotNil(t, state)
}

func TestGeneratePullRequestMapQL(t *testing.T) {
	t.Run("handles no PRs", func(t *testing.T) {
		prMap := internal.GeneratePullRequestMapQL(&genclient.PullRequestsAndStatusResponse{})
		require.Equal(t, map[string]*github.PullRequest{}, prMap)
	})

	t.Run("computes key based on head branch", func(t *testing.T) {
		reviewDecision := fezzik_types.PullRequestReviewDecision_APPROVED
		prMap := internal.GeneratePullRequestMapQL(&genclient.PullRequestsAndStatusResponse{
			Viewer: genclient.PullRequestsAndStatusViewer{
				PullRequests: fezzik_types.PullRequestConnection{
					Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
						{
							Id:          "3",
							Number:      3,
							HeadRefName: "spr/main/0f47588b",
							BaseRefName: "main",
							Title:       "Test PR",
							Body:        "Test Body",
							Mergeable:   fezzik_types.MergeableState_MERGEABLE,
							Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
								Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
									{
										Commit: fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{
											StatusCheckRollup: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup{
												State: fezzik_types.StatusState_SUCCESS,
											},
										},
									},
								},
							},
							ReviewDecision: &reviewDecision,
						},
					},
				},
			},
		})
		expected := map[string]*github.PullRequest{
			"0f47588b": {
				ID:         "3",
				Number:     3,
				FromBranch: "spr/main/0f47588b",
				ToBranch:   "main",
				Title:      "Test PR",
				Body:       "Test Body",
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					NoConflicts:    true,
					ReviewApproved: true,
				},
			},
		}
		require.Equal(t, expected, prMap)
	})
}

func TestComputeCheckStatus(t *testing.T) {
	tests := []struct {
		name           string
		prNode         *fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes
		expectedStatus github.CheckStatus
	}{
		{
			name:           "no status checks",
			prNode:         &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{},
			expectedStatus: github.CheckStatusUnknown,
		},
		{
			name: "checks pass",
			prNode: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
				{
					Commit: fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{
						StatusCheckRollup: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup{
							State: fezzik_types.StatusState_SUCCESS,
						},
					},
				},
			},
			expectedStatus: github.CheckStatusPass,
		}, {
			name: "checks pending",
			prNode: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
				{
					Commit: fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{
						StatusCheckRollup: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup{
							State: fezzik_types.StatusState_PENDING,
						},
					},
				},
			},
			expectedStatus: github.CheckStatusPending,
		}, {
			name: "checks fail",
			prNode: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
				{
					Commit: fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{
						StatusCheckRollup: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommitStatusCheckRollup{
							State: fezzik_types.StatusState_FAILURE,
						},
					},
				},
			},
			expectedStatus: github.CheckStatusFail,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := &github.PullRequestMergeStatus{}
			internal.ComputeCheckStatus(test.prNode, status)
			require.Equal(t, test.expectedStatus, status.ChecksPass)
		})
	}
}

func TestComputeReviewStatus(t *testing.T) {
	tests := []struct {
		name             string
		prNode           *fezzik_types.PullRequestReviewDecision
		expectedApproved bool
	}{
		{
			name:             "no reviews",
			prNode:           nil,
			expectedApproved: false,
		},
		{
			name: "approved",
			prNode: func() *fezzik_types.PullRequestReviewDecision {
				e := fezzik_types.PullRequestReviewDecision_APPROVED
				return &e
			}(),
			expectedApproved: true,
		}, {
			name: "not approved",
			prNode: func() *fezzik_types.PullRequestReviewDecision {
				e := fezzik_types.PullRequestReviewDecision_CHANGES_REQUESTED
				return &e
			}(),
			expectedApproved: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := &github.PullRequestMergeStatus{}
			internal.ComputeReviewStatus(test.prNode, status)
			require.Equal(t, test.expectedApproved, status.ReviewApproved)
		})
	}
}
