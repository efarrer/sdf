package internal_test

import (
	"context"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ejoffe/spr/bl/internal"
	bl "github.com/ejoffe/spr/bl/internal"
	"github.com/ejoffe/spr/bl/ptrutils"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/genqlient"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestNewState(t *testing.T) {
	ctx := context.Background()
	config := config.EmptyConfig()
	prAndStatus := &genqlient.PullRequestsAndStatusResponse{}
	commits := []*object.Commit{}

	state, err := internal.NewState(ctx, config, prAndStatus, commits)
	require.NoError(t, err)
	require.NotNil(t, state)
}

func TestFormatSubject(t *testing.T) {
	require.Equal(t, "                                                  ", internal.FormatSubject(""))
	require.Equal(t, "1234567890                                        ", internal.FormatSubject("1234567890"))
	require.Equal(t, "1234567890123456789012345678901234567890123456 ...", internal.FormatSubject("12345678901234567890123456789012345678901234567890extra"))
}

func TestGetOrphanedPRs(t *testing.T) {
	type testCase struct {
		name            string
		gitCommits      []*internal.LocalCommit
		prMap           map[string]*github.PullRequest
		expectedOrphans mapset.Set[*github.PullRequest]
	}

	pr1 := &github.PullRequest{DatabaseId: "1", Id: "10"}
	pr2 := &github.PullRequest{DatabaseId: "2", Id: "20"}
	pr3 := &github.PullRequest{DatabaseId: "3", Id: "30"}

	testCases := []testCase{
		{
			name: "no orphaned PRs",
			gitCommits: []*internal.LocalCommit{
				{Commit: git.Commit{CommitID: "11111111"}},
				{Commit: git.Commit{CommitID: "22222222"}},
			},
			prMap: map[string]*github.PullRequest{
				"11111111": pr1,
				"22222222": pr2,
			},
			expectedOrphans: mapset.NewSet[*github.PullRequest](),
		},
		{
			name: "one orphaned PR",
			gitCommits: []*internal.LocalCommit{
				{Commit: git.Commit{CommitID: "11111111"}},
			},
			prMap: map[string]*github.PullRequest{
				"11111111": pr1,
				"22222222": pr2,
			},
			expectedOrphans: mapset.NewSet[*github.PullRequest](pr2),
		},
		{
			name: "multiple orphaned PRs",
			gitCommits: []*internal.LocalCommit{
				{Commit: git.Commit{CommitID: "11111111"}},
			},
			prMap: map[string]*github.PullRequest{
				"11111111": pr1,
				"22222222": pr2,
				"33333333": pr3,
			},
			expectedOrphans: mapset.NewSet[*github.PullRequest](pr2, pr3),
		},
		{
			name:            "no PRs",
			gitCommits:      []*internal.LocalCommit{},
			prMap:           map[string]*github.PullRequest{},
			expectedOrphans: mapset.NewSet[*github.PullRequest](),
		},
		{
			name:       "all PRs orphaned",
			gitCommits: []*internal.LocalCommit{},
			prMap: map[string]*github.PullRequest{
				"11111111": pr1,
				"22222222": pr2,
			},
			expectedOrphans: mapset.NewSet[*github.PullRequest](pr1, pr2),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orphanedPRs := internal.GetOrphanedPRs(tc.gitCommits, tc.prMap)
			require.Equal(t, tc.expectedOrphans, orphanedPRs)
		})
	}
}

func TestUpdateRepoToCommitIdToPrSet(t *testing.T) {
	config := config.EmptyConfig()
	config.Repo.GitHubRepoName = t.Name()
	config.State.RepoToCommitIdToPRSet[t.Name()] = map[string]int{
		"11111111": 1,
		"22222222": 0,
		"99999999": 9,
	}
	gitCommits := []*internal.LocalCommit{
		{
			Commit: git.Commit{
				CommitHash: "H1111111",
				CommitID:   "11111111",
			},
		},
		{
			Commit: git.Commit{
				CommitHash: "H2222222",
				CommitID:   "22222222",
			},
		},
		{
			Commit: git.Commit{
				CommitHash: "H3333333",
				CommitID:   "33333333",
			},
		},
	}

	prMap := map[string]*github.PullRequest{
		"11111111": {
			Id:         "10",
			DatabaseId: "1",
		},
		"22222222": {
			Id:         "20",
			DatabaseId: "2",
		},
		"99999999": {
			Id:         "90",
			DatabaseId: "9",
		},
	}

	internal.UpdateRepoToCommitIdToPrSet(config, gitCommits, prMap)

	// Since 99999999 isn't used it should be removed from the mapping
	_, ok := config.State.RepoToCommitIdToPRSet[t.Name()]["99999999"]
	require.False(t, ok)
}

func TestAssignPullRequests(t *testing.T) {
	config := config.EmptyConfig()
	config.Repo.GitHubRepoName = t.Name()
	config.State.RepoToCommitIdToPRSet[t.Name()] = map[string]int{
		"11111111": 1,
		"22222222": 0,
		"99999999": 9,
	}
	gitCommits := []*internal.LocalCommit{
		{
			Commit: git.Commit{
				CommitHash: "LocalH1111111",
				CommitID:   "11111111",
			},
		},
		{
			Commit: git.Commit{
				CommitHash: "LocalH2222222",
				CommitID:   "22222222",
			},
		},
		{
			Commit: git.Commit{
				CommitHash: "LocalH3333333",
				CommitID:   "33333333",
			},
		},
	}

	prMap := map[string]*github.PullRequest{
		"11111111": {
			Id:         "10",
			DatabaseId: "1",
			Commit: git.Commit{
				CommitHash: "RemoteH1111111",
				CommitID:   "11111111",
			},
		},
		"22222222": {
			Id:         "20",
			DatabaseId: "2",
			Commit: git.Commit{
				CommitHash: "RemoteH2222222",
				CommitID:   "22222222",
			},
		},
		"99999999": {
			Id:         "90",
			DatabaseId: "9",
			Commit: git.Commit{
				CommitHash: "RemoteH3333333",
				CommitID:   "33333333",
			},
		},
	}

	internal.AssignPullRequests(config, gitCommits, prMap)

	// The PR is set
	require.Equal(t, "1", gitCommits[0].PullRequest.DatabaseId)
	require.Equal(t, "2", gitCommits[1].PullRequest.DatabaseId)
	require.Equal(t, "10", gitCommits[0].PullRequest.Id)
	require.Equal(t, "20", gitCommits[1].PullRequest.Id)
	require.Nil(t, gitCommits[2].PullRequest)

	// The PRIndex is set
	require.Equal(t, 1, *gitCommits[0].PRIndex)
	require.Equal(t, 0, *gitCommits[1].PRIndex)
	require.Nil(t, gitCommits[2].PRIndex)

	// The PR also has the same commitID
	require.Equal(t, gitCommits[0].CommitID, gitCommits[0].PullRequest.Commit.CommitID)
	require.Equal(t, gitCommits[1].CommitID, gitCommits[1].PullRequest.Commit.CommitID)
}

func TestSetStackedCheck(t *testing.T) {
	config := &config.Config{
		Repo: &config.RepoConfig{
			RequireChecks:   true,
			RequireApproval: true,
		},
	}
	pass := func() *bl.LocalCommit {
		return &bl.LocalCommit{
			Commit: git.Commit{
				WIP: false,
			},
			PullRequest: &github.PullRequest{
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					ReviewApproved: true,
					NoConflicts:    true,
				},
			},
		}
	}
	fail := pass()
	fail.Commit.WIP = true
	commits := []*bl.LocalCommit{
		pass(),
		fail,
		pass(),
	}

	bl.SetStackedCheck(config, commits)
	require.True(t, commits[2].PullRequest.MergeStatus.Stacked)
	require.False(t, commits[1].PullRequest.MergeStatus.Stacked)
	require.False(t, commits[0].PullRequest.MergeStatus.Stacked)
}

func TestApplyIndicies(t *testing.T) {
	// Define the PRs here so the pointer value will be consistent between calls of testingState
	// this allow us to compare sets containing &github.PullRequest
	pr0 := &github.PullRequest{DatabaseId: "0", Id: "00"}
	pr1 := &github.PullRequest{DatabaseId: "1", Id: "10"}
	pr2 := &github.PullRequest{DatabaseId: "2", Id: "20"}
	pr3 := &github.PullRequest{DatabaseId: "3", Id: "30"}
	testingState := func() *internal.State {
		gitCommits := []*internal.LocalCommit{
			{
				Index:       0,
				PRIndex:     ptrutils.Ptr(0),
				PullRequest: pr0,
			},
			{
				Index:       1,
				PRIndex:     ptrutils.Ptr(0),
				PullRequest: pr1,
			},
			{
				Index:       2,
				PRIndex:     ptrutils.Ptr(1),
				PullRequest: pr2,
			},
			{
				Index:       3,
				PRIndex:     ptrutils.Ptr(2),
				PullRequest: pr3,
			},
			{
				Index: 4,
			},
		}
		return &internal.State{
			LocalCommits:  gitCommits,
			OrphanedPRs:   mapset.NewSet[*github.PullRequest](),
			MutatedPRSets: mapset.NewSet[int](),
		}
	}

	tests := []struct {
		desc                       string
		destinationPRIndex         *int
		commitIndex                mapset.Set[int]
		expectedState              func() *internal.State
		expectedDestinationPRINdex *int
	}{
		{
			desc:               "apply to un-PRs commit",
			destinationPRIndex: nil,
			commitIndex:        mapset.NewSet[int](4),
			expectedState: func() *internal.State {
				state := testingState()
				state.LocalCommits[4].PRIndex = ptrutils.Ptr(3)
				state.MutatedPRSets = mapset.NewSet[int](3)
				return state
			},
			expectedDestinationPRINdex: ptrutils.Ptr(3),
		}, {
			desc:               "no-op - update with same PR",
			destinationPRIndex: ptrutils.Ptr(1),
			commitIndex:        mapset.NewSet[int](2),
			expectedState: func() *internal.State {
				state := testingState()
				return state
			},
			expectedDestinationPRINdex: ptrutils.Ptr(1),
		}, {
			desc:               "no-op, - no commits are part of a new PR set",
			destinationPRIndex: nil,
			commitIndex:        mapset.NewSet[int](),
			expectedState: func() *internal.State {
				state := testingState()
				return state
			},
			expectedDestinationPRINdex: nil,
		}, {
			desc:               "merge two PR sets, only needs to mutate the updated set (not the deleted set)",
			destinationPRIndex: ptrutils.Ptr(1),
			commitIndex:        mapset.NewSet[int](2, 3),
			expectedState: func() *internal.State {
				state := testingState()
				state.LocalCommits[3].PRIndex = ptrutils.Ptr(1)
				state.MutatedPRSets = mapset.NewSet[int](1) // Don't need to mutate PRSet 2 as it's been replaced
				return state
			},
			expectedDestinationPRINdex: ptrutils.Ptr(1),
		}, {
			desc:               "split a PR set needs both PR sets updated",
			destinationPRIndex: nil,
			commitIndex:        mapset.NewSet[int](0),
			expectedState: func() *internal.State {
				state := testingState()
				state.LocalCommits[0].PRIndex = ptrutils.Ptr(3)
				state.MutatedPRSets = mapset.NewSet[int](0, 3)
				return state
			},
			expectedDestinationPRINdex: ptrutils.Ptr(3),
		}, {
			desc:               "deleting a PR set adds the existing PRs to the OrphanedPRs",
			destinationPRIndex: ptrutils.Ptr(0),
			commitIndex:        mapset.NewSet[int](),
			expectedState: func() *internal.State {
				state := testingState()
				state.LocalCommits[0].PRIndex = nil
				state.LocalCommits[1].PRIndex = nil
				state.OrphanedPRs.Add(state.LocalCommits[0].PullRequest)
				state.OrphanedPRs.Add(state.LocalCommits[1].PullRequest)
				return state
			},
			expectedDestinationPRINdex: ptrutils.Ptr(0),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			state := testingState()
			indices := internal.Indices{DestinationPRIndex: test.destinationPRIndex, CommitIndexes: test.commitIndex}
			state.ApplyIndices(&indices)

			require.Equal(t, test.expectedState(), state)
			if test.expectedDestinationPRINdex == nil {
				require.Nil(t, indices.DestinationPRIndex)
			} else {
				require.Equal(t, *test.expectedDestinationPRINdex, *indices.DestinationPRIndex)
			}
		})
	}
}

func TestCommitsByPRSet(t *testing.T) {
	// Define the PRs here so the pointer value will be consistent between calls of testingState
	// this allow us to compare sets containing &github.PullRequest
	pr0 := &github.PullRequest{DatabaseId: "0", Id: "00"}
	pr1 := &github.PullRequest{DatabaseId: "1", Id: "10"}
	pr2 := &github.PullRequest{DatabaseId: "2", Id: "20"}
	testingState := internal.State{
		LocalCommits: []*internal.LocalCommit{
			{
				Index:       0,
				PRIndex:     ptrutils.Ptr(0),
				PullRequest: pr0,
			},
			{
				Index:       1,
				PRIndex:     ptrutils.Ptr(1),
				PullRequest: pr1,
			},
			{
				Index:       2,
				PRIndex:     ptrutils.Ptr(2),
				PullRequest: pr2,
			},
			{
				Index:       3,
				PRIndex:     ptrutils.Ptr(0),
				PullRequest: pr0,
			},
			{
				Index: 4,
			},
		},
	}

	commitsByPRSet := testingState.CommitsByPRSet(0)
	require.Len(t, commitsByPRSet, 2)
	require.Equal(t, 0, commitsByPRSet[0].Index)
	require.Equal(t, 3, commitsByPRSet[1].Index)
}

func TestMutatedPRSetsWithOutOfOrderCommits(t *testing.T) {
	// A PR set which is in order is one where the Nth To branch matches the N+1 From branch
	testingState := internal.State{
		LocalCommits: []*internal.LocalCommit{
			// Start PR set 1
			{
				PullRequest: &github.PullRequest{
					ToBranch: "0",
				},
				PRIndex: ptrutils.Ptr(0),
			},
			{
				PullRequest: &github.PullRequest{
					FromBranch: "0",
					ToBranch:   "1",
				},
				PRIndex: ptrutils.Ptr(0),
			},
			{
				// Just a commit without a PR
			},
			// Start PR set 1 which is out of order
			{
				PullRequest: &github.PullRequest{
					ToBranch: "0",
				},
				PRIndex: ptrutils.Ptr(1),
			},
			{
				PullRequest: &github.PullRequest{
					FromBranch: "1",
				},
				PRIndex: ptrutils.Ptr(1),
			},
			// Resume PR set 0
			{
				PullRequest: &github.PullRequest{
					FromBranch: "1",
					ToBranch:   "2",
				},
				PRIndex: ptrutils.Ptr(0),
			},
		},
	}

	testingState.MutatedPRSets = mapset.NewSet[int](0)
	require.True(t, testingState.MutatedPRSetsWithOutOfOrderCommits().IsEmpty())
	testingState.MutatedPRSets = mapset.NewSet[int](1)
	require.True(t, testingState.MutatedPRSetsWithOutOfOrderCommits().Contains(1))
}

func TestPullRequests(t *testing.T) {
	pr0 := &github.PullRequest{DatabaseId: "0", Id: "00"}
	pr1 := &github.PullRequest{DatabaseId: "1", Id: "10"}
	pr2 := &github.PullRequest{DatabaseId: "2", Id: "20"}
	pr3 := &github.PullRequest{DatabaseId: "3", Id: "30"}
	testingCommits := []*internal.LocalCommit{
		{
			Index:       0,
			PRIndex:     ptrutils.Ptr(0),
			PullRequest: pr0,
		},
		{
			Index:       1,
			PRIndex:     ptrutils.Ptr(0),
			PullRequest: pr1,
		},
		{
			Index:       2,
			PRIndex:     ptrutils.Ptr(1),
			PullRequest: pr2,
		},
		{
			Index:       3,
			PRIndex:     ptrutils.Ptr(2),
			PullRequest: pr3,
		},
		{
			Index: 4,
		},
	}

	expectedPullRequests := []*github.PullRequest{
		pr0, pr1, pr2, pr3,
	}
	pullRequests := internal.PullRequests(testingCommits)
	require.Equal(t, expectedPullRequests, pullRequests)
}

func TestGeneratePullRequestMap(t *testing.T) {
	t.Run("handles no PRs", func(t *testing.T) {
		prMap := internal.GeneratePullRequestMap(&genqlient.PullRequestsAndStatusResponse{})
		require.Equal(t, map[string]*github.PullRequest{}, prMap)
	})

	t.Run("computes key based on head branch", func(t *testing.T) {
		prMap := internal.GeneratePullRequestMap(&genqlient.PullRequestsAndStatusResponse{
			Viewer: genqlient.PullRequestsAndStatusViewerUser{
				PullRequests: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnection{
					Nodes: []genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
						{
							Id:          "30",
							DatabaseId:  3,
							Number:      3,
							HeadRefName: "spr/main/0f47588b",
							BaseRefName: "main",
							Title:       "Test PR",
							Body:        "Test Body",
							Mergeable:   genqlient.MergeableStateMergeable,
							Commits: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnection{
								Nodes: []genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommit{
									{
										Commit: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit{
											Oid:             "012345",
											MessageHeadline: "WIP Headline",
											MessageBody:     "some message",
											StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommitStatusCheckRollup{
												State: genqlient.StatusStateSuccess,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		})
		expected := map[string]*github.PullRequest{
			"0f47588b": {
				Id:         "30",
				DatabaseId: "3",
				Number:     3,
				FromBranch: "spr/main/0f47588b",
				ToBranch:   "main",
				Title:      "Test PR",
				Body:       "Test Body",
				MergeStatus: github.PullRequestMergeStatus{
					ChecksPass:     github.CheckStatusPass,
					NoConflicts:    true,
					ReviewApproved: false,
				},
				Commit: git.Commit{
					CommitID:   "0f47588b",
					CommitHash: "012345",
					Subject:    "WIP Headline",
					Body:       "some message",
					WIP:        true,
				},
			},
		}
		require.Equal(t, expected, prMap)
	})
}

func TestCommitIdFromBranch(t *testing.T) {
	require.Equal(t, "", bl.CommitIdFromBranch(""))
	require.Equal(t, "", bl.CommitIdFromBranch("spr/"))
	require.Equal(t, "", bl.CommitIdFromBranch("spr/main"))
	require.Equal(t, "", bl.CommitIdFromBranch("spr/main/1234444"))
	require.Equal(t, "", bl.CommitIdFromBranch("other/main/12344448"))
	require.Equal(t, "12344448", bl.CommitIdFromBranch("spr/main/12344448"))
}

func TestComputeMergeStatus(t *testing.T) {
	tests := []struct {
		desc     string
		pr       genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest
		expected github.PullRequestMergeStatus
	}{
		{
			desc: "all pass",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestStatusCheckRollup{
					State: genqlient.StatusStateSuccess,
				},
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
			},
		},
		{
			desc: "no state",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestStatusCheckRollup{
					State: "",
				},
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    true,
			},
		},
		{
			desc: "check error",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestStatusCheckRollup{
					State: genqlient.StatusStateError,
				},
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusFail,
				ReviewApproved: true,
				NoConflicts:    true,
			},
		},
		{
			desc: "check fail",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestStatusCheckRollup{
					State: genqlient.StatusStateFailure,
				},
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusFail,
				ReviewApproved: true,
				NoConflicts:    true,
			},
		},
		{
			desc: "check pending",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				StatusCheckRollup: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestStatusCheckRollup{
					State: genqlient.StatusStatePending,
				},
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPending,
				ReviewApproved: true,
				NoConflicts:    true,
			},
		},
		{
			desc: "conflicts",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				Mergeable:      genqlient.MergeableStateConflicting,
				ReviewDecision: genqlient.PullRequestReviewDecisionApproved,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: true,
				NoConflicts:    false,
			},
		},
		{
			desc: "review required",
			pr: genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest{
				Mergeable:      genqlient.MergeableStateMergeable,
				ReviewDecision: genqlient.PullRequestReviewDecisionReviewRequired,
			},
			expected: github.PullRequestMergeStatus{
				ChecksPass:     github.CheckStatusPass,
				ReviewApproved: false,
				NoConflicts:    true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			status := internal.ComputeMergeStatus(test.pr)
			require.Equal(t, test.expected, status)
		})
	}
}

func TestGenerateCommits_LinksCommitsAndSetsIndicies(t *testing.T) {
	commits := bl.GenerateCommits(
		[]*object.Commit{
			{
				Hash:    plumbing.NewHash("01"),
				Message: "commit-id:11111111",
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("02"),
				},
			},
			{
				Hash:    plumbing.NewHash("02"),
				Message: "commit-id:22222222",
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("03"),
				},
			},
			{
				Hash:    plumbing.NewHash("03"),
				Message: "commit-id:33333333",
			},
		},
	)

	require.Equal(t, 2, commits[0].Index)

	require.Equal(t, "11111111", commits[0].CommitID)
}

func TestUpdatePRSetState(t *testing.T) {
	config := config.EmptyConfig()
	config.Repo.GitHubRepoName = t.Name()
	config.State.RepoToCommitIdToPRSet["other"] = map[string]int{
		"44444444": 4,
	}
	config.State.RepoToCommitIdToPRSet[t.Name()] = map[string]int{
		"11111111": 1,
		"22222222": 0,
		"99999999": 9,
	}

	testingCommits := []*internal.LocalCommit{
		{
			Commit: git.Commit{
				CommitID: "11111111",
			},
			PRIndex: ptrutils.Ptr(0),
		},
		{
			Commit: git.Commit{
				CommitID: "22222222",
			},
			PRIndex: ptrutils.Ptr(0),
		},
		{
			Commit: git.Commit{
				CommitID: "33333333",
			},
			PRIndex: ptrutils.Ptr(1),
		},
		{
			Commit: git.Commit{
				CommitID: "44444444",
			},
			PRIndex: ptrutils.Ptr(2),
		},
		{
			Commit: git.Commit{
				CommitID: "55555555",
			},
		},
	}

	expectedStateMap := map[string]map[string]int{
		"other": {
			"44444444": 4,
		},
		t.Name(): {
			"11111111": 0,
			"22222222": 0,
			"33333333": 1,
			"44444444": 2,
		},
	}

	state := internal.State{LocalCommits: testingCommits}

	state.UpdatePRSetState(config)

	require.Equal(t, expectedStateMap, config.State.RepoToCommitIdToPRSet)

}

func TestHeadFirst(t *testing.T) {
	t.Run("preserves HEAD first", func(t *testing.T) {
		res := bl.HeadFirst([]*object.Commit{
			{
				Hash:    plumbing.NewHash("01"),
				Message: "HEAD",
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("03"),
					plumbing.NewHash("02"),
				},
			},
			{
				Hash: plumbing.NewHash("02"),
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("04"),
					plumbing.NewHash("05"),
				},
			},
		})
		require.Equal(t, "HEAD", res[0].Message)
	})

	t.Run("sorts HEAD first", func(t *testing.T) {
		res := bl.HeadFirst([]*object.Commit{
			{
				Hash: plumbing.NewHash("02"),
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("04"),
					plumbing.NewHash("05"),
				},
			},
			{
				Hash:    plumbing.NewHash("01"),
				Message: "HEAD",
				ParentHashes: []plumbing.Hash{
					plumbing.NewHash("03"),
					plumbing.NewHash("02"),
				},
			},
		})
		require.Equal(t, "HEAD", res[0].Message)
	})
}

func TestCommitId(t *testing.T) {
	require.Equal(t, "c0530239", bl.CommitId("msg\nsdf\ncommit-id:c0530239"))
	require.Equal(t, "c0530239", bl.CommitId("msg\nsdf\ncommit-id:c0530239\nasdf"))
	require.Equal(t, "c0530239", bl.CommitId("commit-id:c0530239"))
	require.Equal(t, "", bl.CommitId("commit-id:c053023999")) // extra character
	require.Equal(t, "", bl.CommitId("xcommit-id:c0530239"))
	require.Equal(t, "", bl.CommitId(""))
	require.Equal(t, "", bl.CommitId("\n\ncommit-id:"))
}

func TestIsWIP(t *testing.T) {
	require.True(t, bl.IsWIP("WIP\nsother text"))
	require.True(t, bl.IsWIP("[WIP]\nsother text"))
	require.False(t, bl.IsWIP("nop\nsother text"))
}

func TestSubject(t *testing.T) {
	require.Equal(t, "msg", bl.Subject("msg\nsdf\nsdf"))
	require.Equal(t, "msg", bl.Subject("msg\nsdf"))
	require.Equal(t, "msg", bl.Subject("msg\n"))
	require.Equal(t, "msg", bl.Subject("msg"))
	require.Equal(t, "", bl.Subject("\nmsg"))
	require.Equal(t, "", bl.Subject(""))
}

func TestBody(t *testing.T) {
	require.Equal(t, "sdf\nsdf", bl.Body("msg\nsdf\nsdf"))
	require.Equal(t, "sdf", bl.Body("msg\nsdf"))
	require.Equal(t, "", bl.Body("msg\n"))
	require.Equal(t, "", bl.Body("msg"))
	require.Equal(t, "msg", bl.Body("\nmsg"))
	require.Equal(t, "", bl.Body(""))
}
