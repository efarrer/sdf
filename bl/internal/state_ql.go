package internal

import (
	"context"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/fezzik_types"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func NewReadStateQL(ctx context.Context, config *config.Config, gitcmd git.GitInterface, client genclient.Client) (*State, error) {
	prAndStatus, err := client.PullRequestsAndStatus(ctx, config.Repo.GitHubRepoOwner, config.Repo.GitHubRepoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests and status: %w", err)
	}

	commits, err := gitcmd.UnMergedCommits(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get unmerged commits: %w", err)
	}

	return NewStateQL(ctx, config, prAndStatus, commits)
}

func NewStateQL(ctx context.Context, config *config.Config, prAndStatus *genclient.PullRequestsAndStatusResponse, commits []*object.Commit) (*State, error) {
	prMap := GeneratePullRequestMapQL(prAndStatus)

	gitCommits := GenerateCommits(commits)
	for _, gitCommit := range gitCommits {
		gitCommit.PullRequest = prMap[gitCommit.CommitID]
	}

	orphanedPRs := AssignPullRequests(config, gitCommits, prMap)

	SetStackedCheck(config, gitCommits)

	return &State{
		Commits:       gitCommits,
		OrphanedPRs:   orphanedPRs,
		MutatedPRSets: mapset.NewSet[int](),
	}, nil
}

func GeneratePullRequestMapQL(prAndStatus *genclient.PullRequestsAndStatusResponse) map[string]*github.PullRequest {
	if prAndStatus == nil || prAndStatus.Viewer.PullRequests.Nodes == nil {
		return map[string]*github.PullRequest{}
	}

	prMap := map[string]*github.PullRequest{}

	for _, prNode := range *prAndStatus.Viewer.PullRequests.Nodes {
		commitID := CommitIdFromBranch(prNode.HeadRefName)
		if commitID == "" {
			continue
		}

		mergeStatus := github.PullRequestMergeStatus{}
		ComputeCheckStatus(prNode.Commits.Nodes, &mergeStatus)
		ComputeReviewStatus(prNode.ReviewDecision, &mergeStatus)
		ghpr := &github.PullRequest{
			ID:          prNode.Id,
			Number:      prNode.Number,
			FromBranch:  prNode.HeadRefName,
			ToBranch:    prNode.BaseRefName,
			Title:       prNode.Title,
			Body:        prNode.Body,
			MergeStatus: mergeStatus,
		}
		ghpr.MergeStatus.NoConflicts = prNode.Mergeable == fezzik_types.MergeableState_MERGEABLE
		prMap[commitID] = ghpr
	}

	return prMap
}

func ComputeCheckStatus(prCommitsNodes *fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes, prms *github.PullRequestMergeStatus) {
	if len(*prCommitsNodes) > 0 && (*prCommitsNodes)[0].Commit.StatusCheckRollup != nil {
		switch (*prCommitsNodes)[0].Commit.StatusCheckRollup.State {
		case fezzik_types.StatusState_SUCCESS:
			prms.ChecksPass = github.CheckStatusPass
		case fezzik_types.StatusState_PENDING:
			prms.ChecksPass = github.CheckStatusPending
		case fezzik_types.StatusState_ERROR, fezzik_types.StatusState_FAILURE:
			prms.ChecksPass = github.CheckStatusFail
		default:
			prms.ChecksPass = github.CheckStatusUnknown
		}
	} else {
		prms.ChecksPass = github.CheckStatusUnknown
	}
}

func ComputeReviewStatus(reviewDecision *fezzik_types.PullRequestReviewDecision, prms *github.PullRequestMergeStatus) {
	if reviewDecision != nil && *reviewDecision == fezzik_types.PullRequestReviewDecision_APPROVED {
		prms.ReviewApproved = true
	}
}
