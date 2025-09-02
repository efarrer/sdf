package internal

import (
	"context"
	"fmt"
	"iter"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ejoffe/spr/bl/maputils"
	"github.com/ejoffe/spr/bl/ptrutils"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/genqlient"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// A LocalCommit is a commit its associated Pull Request, and metadata.
// Note that the github.PullRequest also might have a copy of the commit and that commit will have different hash as it
// when is pushed to the remote repo.
type LocalCommit struct {
	git.Commit

	// The pull request that has this commit at the top
	PullRequest *github.PullRequest

	// The index is a simple way of referring to a commit. Child commits have larger indices.
	Index int

	// The PRIndex is a simple way of referring to a set of Pull Requests. A nil PRIndex indicates that the commit doesn't
	// have a PR (that was created by spr).
	PRIndex *int
}

// Indices is a list of commit indices and the destination pull request set index
type Indices struct {
	DestinationPRIndex *int            // Matches LocalCommit.PRIndex
	CommitIndexes      mapset.Set[int] // Matches LocalCommit.Index
}

// State holds the state of the local commits and PRs
type State struct {
	RepositoryId string
	// The 0th commit in this slice is the HEAD commit
	LocalCommits  []*LocalCommit
	OrphanedPRs   mapset.Set[*github.PullRequest]
	MutatedPRSets mapset.Set[int]
}

func indexColor(i *int) string {
	if i == nil {
		return github.ColorBlue
	}
	switch *i % 4 {
	case 0:
		return github.ColorRed
	case 1:
		return github.ColorGreen
	case 2:
		return github.ColorBlue
	case 3:
		return github.ColorLightBlue
	}
	return github.ColorReset
}

func padNumber(pad int) func(string) string {
	return func(s string) string {
		padding := pad - len(s)
		if padding > 0 {
			s += strings.Repeat(" ", padding)
		}
		return s
	}
}

func FormatSubject(subject string) string {
	length := 50
	runeCount := utf8.RuneCountInString(subject)
	if runeCount <= length {
		return padNumber(length)(subject)
	}

	maxLength := 46

	var truncated string
	count := 0
	for _, r := range subject {
		truncated += string(r)
		count++
		if count == maxLength {
			break
		}
	}

	return truncated + " ..."
}

func (prc LocalCommit) PRSetString(config *config.Config) string {
	noPrMessage := "No Pull Request Created"
	empty := github.StatusBitIcons(config)["empty"]

	prString := fmt.Sprintf("[%s%s%s%s] %s : %s",
		empty,
		empty,
		empty,
		empty,
		FormatSubject(prc.Commit.Subject),
		noPrMessage,
	)

	if prc.PullRequest != nil {
		padding := padNumber(5)
		prInfo := fmt.Sprintf("%s %s : https://%s/%s/%s/pull/%s",
			prc.PullRequest.StatusString(config),
			FormatSubject(prc.Commit.Subject),
			config.Repo.GitHubHost, config.Repo.GitHubRepoOwner, config.Repo.GitHubRepoName, padding(fmt.Sprintf("%d", prc.PullRequest.Number)))
		prString = prInfo
	}

	prIndex := "--"
	if prc.PRIndex != nil {
		prIndex = fmt.Sprintf("s%d", *prc.PRIndex)
	}

	line := fmt.Sprintf("%s%2d%s %s%s%s %s",
		github.ColorLightBlue,
		prc.Index,
		github.ColorReset,
		indexColor(prc.PRIndex),
		prIndex,
		github.ColorReset,
		//FormatSubject(prc.Commit.Subject),
		prString,
	)

	return github.TrimToTerminal(config, line)
}

// Generic function to convert a nil pointer to its zero value.
// Works for any type.
func derefOrDefault[T any](ptr *T) T {
	if ptr == nil {
		var zero T // Zero value of the type T
		return zero
	}
	return *ptr
}

// NewReadState pulls git and github information and constructs the state of the local unmerged commits.
// The resulting State contains the ordered and linked commits along with their associated PRs
func NewReadState(ctx context.Context, config *config.Config, gitcmd git.GitInterface, github github.GitHubInterface) (*State, error) {
	prAndStatus, err := github.PullRequestsAndStatus(ctx, config.Repo.GitHubRepoOwner, config.Repo.GitHubRepoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests and status: %w", err)
	}

	commits, err := gitcmd.UnMergedCommits(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get unmerged commits: %w", err)
	}

	return NewState(ctx, config, prAndStatus, commits)
}

// NewState composes git and github information and constructs the state of the local unmerged commits.
// The resulting State contains the ordered and linked commits along with their associated PRs
func NewState(ctx context.Context, config *config.Config, prAndStatus *genqlient.PullRequestsAndStatusResponse, commits []*object.Commit) (*State, error) {
	prMap := GeneratePullRequestMap(prAndStatus)

	gitCommits := GenerateCommits(commits)
	for _, gitCommit := range gitCommits {
		gitCommit.PullRequest = prMap[gitCommit.CommitID]
	}

	orphanedPRs := GetOrphanedPRs(gitCommits, prMap)
	UpdateRepoToCommitIdToPrSet(config, gitCommits, prMap)
	AssignPullRequests(config, gitCommits, prMap)

	SetStackedCheck(config, gitCommits)

	return &State{
		RepositoryId:  prAndStatus.Repository.Id,
		LocalCommits:  gitCommits,
		OrphanedPRs:   orphanedPRs,
		MutatedPRSets: mapset.NewSet[int](),
	}, nil
}

// GetOrphanedPRs gets all PRs that reference commits that aren't in the unmerged-commits
func GetOrphanedPRs(
	gitCommits []*LocalCommit,
	prMap map[string]*github.PullRequest,
) mapset.Set[*github.PullRequest] {
	// Add PRs that reference commits that are not part of the unmerged-commits to the orphans list
	prGCMap := maputils.NewGC(prMap)

	for _, gitCommit := range gitCommits {
		prGCMap.Lookup(gitCommit.CommitID)
	}

	orphanedPrs := mapset.NewSet[*github.PullRequest]()
	for _, v := range prGCMap.GetUnaccessed() {
		orphanedPrs.Add(v)
	}

	return orphanedPrs
}

func UpdateRepoToCommitIdToPrSet(
	config *config.Config,
	gitCommits []*LocalCommit,
	prMap map[string]*github.PullRequest,
) {
	// Get the mapping of commitIds to PR Sets
	prSetMap, ok := config.State.RepoToCommitIdToPRSet[config.Repo.GitHubRepoName]
	if !ok {
		prSetMap = map[string]int{}
	}
	// Purge any mappings that aren't used
	purgeMap := maputils.NewGC(prSetMap)

	for _, gitCommit := range gitCommits {
		if _, ok := prMap[gitCommit.CommitID]; ok {
			purgeMap.Lookup(gitCommit.CommitID)
		}
	}

	config.State.RepoToCommitIdToPRSet[config.Repo.GitHubRepoName] = purgeMap.PurgeUnaccessed()
}

func AssignPullRequests(
	config *config.Config,
	gitCommits []*LocalCommit,
	prMap map[string]*github.PullRequest,
) {
	// Get the mapping of commitIds to PR Set
	prSetMap, ok := config.State.RepoToCommitIdToPRSet[config.Repo.GitHubRepoName]
	if !ok {
		prSetMap = map[string]int{}
	}
	for _, gitCommit := range gitCommits {
		if pr, ok := prMap[gitCommit.CommitID]; ok {
			if prIndex, ok := prSetMap[gitCommit.CommitID]; ok {
				gitCommit.PRIndex = ptrutils.Ptr(prIndex)
			}
			gitCommit.PullRequest = pr
		}
	}
}

func SetStackedCheck(config *config.Config, gitCommits []*LocalCommit) {
	for i := len(gitCommits) - 1; i >= 0; i-- {
		cm := gitCommits[i]
		if cm.PullRequest == nil {
			continue
		}
		if cm.WIP {
			return
		}
		if !cm.PullRequest.MergeStatus.NoConflicts {
			return
		}
		if config.Repo.RequireChecks {
			if cm.PullRequest.MergeStatus.ChecksPass != github.CheckStatusPass {
				return
			}
		}
		if config.Repo.RequireApproval {
			if !cm.PullRequest.MergeStatus.ReviewApproved {
				return
			}
		}
		cm.PullRequest.MergeStatus.Stacked = true
	}
}

func (s *State) LocalCommitsIter() iter.Seq[*LocalCommit] {
	return slices.Values(s.LocalCommits)
}

// Returns the HEAD commit
func (s *State) Head() *LocalCommit {
	if len(s.LocalCommits) == 0 {
		return nil
	}
	return s.LocalCommits[0]
}

// ApplyIndices applies the commits in state and updates the State's mutatedPRSets
// The Indices.DestinationPRIndex is update if needed
func (s *State) ApplyIndices(indices *Indices) {
	// If we're assigning 0 commits to a new PR (DestinationPRIndex == nil) then do nothing
	if indices.DestinationPRIndex == nil && indices.CommitIndexes.Cardinality() == 0 {
		return
	}
	// If DestinationPRIndex is null find the next available PR index and update DestinationPRIndex
	if indices.DestinationPRIndex == nil {
		nextDestinationPRIndex := 0
		for _, cm := range s.LocalCommits {
			if cm.PRIndex != nil && *cm.PRIndex >= nextDestinationPRIndex {
				nextDestinationPRIndex = *cm.PRIndex + 1
			}
		}

		indices.DestinationPRIndex = &nextDestinationPRIndex
	}

	// iterate over the commits and update the PRIndex for all matching commitIndex
	// clear the PRs for existing PRs that are in the PRIndex but not in the commitIndex
	for _, cm := range s.LocalCommits {
		shouldBeInPrSet := indices.CommitIndexes.Contains(cm.Index)
		isInPrSet := cm.PRIndex != nil && *cm.PRIndex == *indices.DestinationPRIndex

		// If the commit is already in the PR set and it should be in the PR set then we are done
		if isInPrSet && shouldBeInPrSet {
			continue
		}
		// If the commit is **not** already in the PR set and it should **not** be in the PR set then we are done
		if !isInPrSet && !shouldBeInPrSet {
			continue
		}
		// If the commit is already in the PR set and it should **not** be then we need to clear the PR Index
		if isInPrSet && !shouldBeInPrSet {
			s.OrphanedPRs.Add(cm.PullRequest)
			s.MutatedPRSets.Add(*indices.DestinationPRIndex)
			cm.PRIndex = nil
			continue
		}

		// If the commit is **not** already in the PR set and it should be then we need to set PR Index
		if !isInPrSet && shouldBeInPrSet {
			// If we are replacing another PR then both the old and the new PR sets were mutated
			if cm.PRIndex != nil {
				s.MutatedPRSets.Add(*cm.PRIndex)
			}
			s.MutatedPRSets.Add(*indices.DestinationPRIndex)
			cm.PRIndex = indices.DestinationPRIndex
		}
	}

	// It is possible to mutate a PR set out of existence. So purge any in the MutatedPRSets that no longer exist.
	existingPRSets := mapset.NewSet[int]()
	for _, cm := range s.LocalCommits {
		if cm.PRIndex == nil {
			continue
		}
		existingPRSets.Add(*cm.PRIndex)
	}
	s.MutatedPRSets = s.MutatedPRSets.Intersect(existingPRSets)
}

// CommitsByPRSet returns all of the commits for the given PR set with the newest commits first.
// Note that the Index fields are not changed in the returned LocalCommits
func (s *State) CommitsByPRSet(prIndex int) []*LocalCommit {
	var commits []*LocalCommit
	for _, ci := range s.LocalCommits {
		if ci.PRIndex == nil {
			continue
		}

		if *ci.PRIndex == prIndex {
			commits = append(commits, ci)
		}
	}

	return commits
}

// MutatedPRSetsWithOutOfOrderCommits returns the PRSets where the commits are out of order and the PRs need to be rebuilt.
func (s *State) MutatedPRSetsWithOutOfOrderCommits() mapset.Set[int] {
	outOfOrderPRSets := mapset.NewSet[int]()
	for prSet := range s.MutatedPRSets.Iter() {
		lastTo := ""

		for _, commit := range s.LocalCommits {
			// If the commit doesn't have a PR then we can ignore it.
			if commit.PullRequest == nil {
				continue
			}
			// Same as above.
			if commit.PRIndex == nil {
				continue
			}
			// If the commit is a part of a different PR set then ignore it.
			if *commit.PRIndex != prSet {
				continue
			}

			if lastTo == "" {
				lastTo = commit.PullRequest.ToBranch
				continue
			}
			if commit.PullRequest.FromBranch != lastTo {
				outOfOrderPRSets.Add(prSet)
				break
			}
			lastTo = commit.PullRequest.ToBranch
		}
	}
	return outOfOrderPRSets
}

// PullRequest gets all pull request from the LocalCommits.
func PullRequests(commits []*LocalCommit) []*github.PullRequest {
	pullRequests := make([]*github.PullRequest, 0, len(commits))
	for _, ci := range commits {
		if ci.PullRequest != nil {
			pullRequests = append(pullRequests, ci.PullRequest)
		}
	}
	return pullRequests
}

// GeneratePullRequestMap creates a mapping of commit-id:####### to the github.PullRequst for that commit
func GeneratePullRequestMap(prAndStatus *genqlient.PullRequestsAndStatusResponse) map[string]*github.PullRequest {
	if prAndStatus == nil || prAndStatus.Viewer.PullRequests.Nodes == nil {
		return map[string]*github.PullRequest{}
	}

	prMap := map[string]*github.PullRequest{}

	for _, prNode := range prAndStatus.Viewer.PullRequests.Nodes {
		commitID := CommitIdFromBranch(prNode.HeadRefName)
		if commitID == "" {
			continue
		}

		var commit genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequestCommitsPullRequestCommitConnectionNodesPullRequestCommitCommit
		if len(prNode.Commits.Nodes) > 0 {
			commit = prNode.Commits.Nodes[0].Commit
		}

		ghpr := &github.PullRequest{
			Id:          prNode.Id,
			DatabaseId:  fmt.Sprintf("%d", prNode.DatabaseId),
			Number:      prNode.Number,
			FromBranch:  prNode.HeadRefName,
			ToBranch:    prNode.BaseRefName,
			Title:       prNode.Title,
			Body:        prNode.Body,
			MergeStatus: ComputeMergeStatus(prNode),
			Commit: git.Commit{
				CommitID:   commitID,
				CommitHash: commit.Oid,
				Subject:    commit.MessageHeadline,
				Body:       commit.MessageBody,
				WIP:        IsWIP(commit.MessageHeadline),
			},
		}
		prMap[commitID] = ghpr
	}

	return prMap
}

func CommitIdFromBranch(branchName string) string {
	segments := strings.Split(branchName, "/")
	if len(segments) != 3 {
		return ""
	}
	if segments[0] != "spr" {
		return ""
	}
	commitId := segments[2]
	if len(commitId) != 8 {
		return ""
	}
	return commitId
}

func ComputeMergeStatus(pr genqlient.PullRequestsAndStatusViewerUserPullRequestsPullRequestConnectionNodesPullRequest) github.PullRequestMergeStatus {
	prms := github.PullRequestMergeStatus{}
	switch pr.StatusCheckRollup.State {
	case genqlient.StatusStateError:
		fallthrough
	case genqlient.StatusStateFailure:
		prms.ChecksPass = github.CheckStatusFail
	case genqlient.StatusStateExpected:
		fallthrough
	case genqlient.StatusStatePending:
		prms.ChecksPass = github.CheckStatusPending
	case "":
		fallthrough
	case genqlient.StatusStateSuccess:
		prms.ChecksPass = github.CheckStatusPass
	}

	prms.NoConflicts = pr.Mergeable == genqlient.MergeableStateMergeable
	prms.ReviewApproved = pr.ReviewDecision == genqlient.PullRequestReviewDecisionApproved

	return prms
}

// GenerateCommits transforms a []*object.Commit to a []*LocalCommit
func GenerateCommits(commits []*object.Commit) []*LocalCommit {
	gitCommits := make([]*LocalCommit, 0, len(commits))

	// Make sure that commits are always stored HEAD first.
	commits = HeadFirst(commits)

	for i, cm := range commits {
		commitId := CommitId(cm.Message)

		c := &LocalCommit{
			Commit: git.Commit{
				CommitID:   commitId,
				CommitHash: cm.Hash.String(),
				Subject:    Subject(cm.Message),
				Body:       Body(cm.Message),
				WIP:        IsWIP(cm.Message),
			},
			PullRequest: nil,
			Index:       len(commits) - (i + 1),
			PRIndex:     nil,
		}
		gitCommits = append(gitCommits, c)
	}

	return gitCommits
}

// UpdatePRSetState updates the RepoToCommitIdToPRSet in the config based upon the state.Commits
func (s *State) UpdatePRSetState(config *config.Config) {
	// It is simpler to just build up a new map for this repo than to mutate the existing map
	prSetMap := map[string]int{}

	for _, commit := range s.LocalCommits {
		if commit.PRIndex == nil {
			continue
		}
		prSetMap[commit.CommitID] = *commit.PRIndex

	}
	config.State.RepoToCommitIdToPRSet[config.Repo.GitHubRepoName] = prSetMap
}

func HeadFirst(commits []*object.Commit) []*object.Commit {
	if len(commits) < 2 {
		return commits
	}

	// See if the second is listed as the firsts parent if so we are in the right order
	for _, firstParents := range commits[0].ParentHashes {
		if commits[1].Hash.String() == firstParents.String() {
			return commits
		}
	}
	slices.Reverse(commits)
	return commits
}

var commitIDRegex = regexp.MustCompile(`(?m)^commit-id\:([a-f0-9]{8})$`)

// CommitId parses out the commit id from "commit-id:00000000"
func CommitId(msg string) string {
	matches := commitIDRegex.FindStringSubmatch(msg)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// IsWIP returns true if the message starts with "WIP"
func IsWIP(msg string) bool {
	return strings.HasPrefix(msg, "WIP") || strings.HasPrefix(msg, "[WIP]")
}

// Subject returns the first line of the message
func Subject(msg string) string {
	return strings.SplitN(msg, "\n", 2)[0]
}

// Subject returns all but the first line of the message
func Body(msg string) string {
	res := strings.SplitN(msg, "\n", 2)
	if len(res) < 2 {
		return ""
	}
	return res[1]
}
