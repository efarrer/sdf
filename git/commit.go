package git

import (
	"context"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitInterface interface {
	AppendCommitId() error
	GitWithEditor(args string, output *string, editorCmd string) error
	Git(args string, output *string) error
	MustGit(args string, output *string)
	RootDir() string
	DeleteRemoteBranch(ctx context.Context, branch string) error
	GetLocalBranchShortName() (string, error)
	Fetch(remoteName string, prune bool) error
	Reference(name string, resolved bool) (string, error)
	Push(remoteName string, refspecs []string) error
	RemoteBranches() (mapset.Set[string], error)
	BranchExists(branchName string) (bool, error)
	OriginMainRef(ctx context.Context) (string, error)
	OriginBranchRef(ctx context.Context, branch string) (string, error)
	UnMergedCommits(ctx context.Context) ([]*object.Commit, error)
	Rebase(ctx context.Context, remoteName, branchName string) error
	Email() (string, error)
}

// Commit has all the git commit info
type Commit struct {
	// CommitID is a long lasting id describing the commit.
	//  The CommitID is generated and added to the end of the commit message on the initial commit.
	//  The CommitID remains the same when a commit is amended.
	CommitID string

	// CommitHash is the git commit hash, this gets updated everytime the commit is amended.
	CommitHash string

	// Subject is the subject of the commit message.
	Subject string

	// Body is the body of the commit message.
	Body string

	// WIP is true if the commit is still work in progress.
	WIP bool
}
