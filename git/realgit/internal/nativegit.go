package internal

import (
	"context"
	"fmt"
	"os"

	mapset "github.com/deckarep/golang-set/v2"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gogitplumbing "github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// Repo creates a *gogit.Repository the *gogit.Repository should not be shared between goroutines
func Repo() *gogit.Repository {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	repo, err := gogit.PlainOpenWithOptions(cwd, &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		fmt.Printf("%s is not a git repository\n", cwd)
		os.Exit(2)
	}

	return repo
}

type NativeGit struct {
	base *Gitbase
}

func NewNativeGit(base *Gitbase) NativeGit {
	return NativeGit{base: base}
}

func (c NativeGit) repo() *gogit.Repository {
	return Repo()
}

func (c NativeGit) DeleteRemoteBranch(ctx context.Context, branch string) error {
	remoteName := c.base.Config.Repo.GitHubRemote

	remote, err := c.repo().Remote(remoteName)
	if err != nil {
		return fmt.Errorf("getting remote %s %w", remoteName, err)
	}

	// Construct the reference name for branch
	refName := gogitplumbing.NewBranchReferenceName(branch)

	pushOptions := gogit.PushOptions{
		RemoteName: remoteName,
		// Nothing before the colon says to push nothing to the destination branch (which deletes it).
		RefSpecs: []gogitconfig.RefSpec{gogitconfig.RefSpec(fmt.Sprintf(":%s", refName))},
	}

	// Delete the remote branch
	err = remote.Push(&pushOptions)
	if err != nil {
		return fmt.Errorf("removing remote branch %s %w", branch, err)
	}

	return nil
}

// GetLocalBranchShortName returns the local branch short name (like "main")
func (c NativeGit) GetLocalBranchShortName() (string, error) {
	ref, err := c.repo().Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD %w", err)
	}

	return string(ref.Name().Short()), nil
}

// Fetch fetches references along with the objects necessary to complete
// their histories, from the remote named as FetchOptions.RemoteName.
func (c NativeGit) Fetch(remoteName string, prune bool) error {
	return c.repo().Fetch(&gogit.FetchOptions{
		RemoteName: remoteName,
		Prune:      prune,
	})
}

// Reference returns the reference for a given reference name. If resolved is
// true, any symbolic reference will be resolved.
func (c NativeGit) Reference(name string, resolved bool) (string, error) {
	ref, err := c.repo().Reference(plumbing.ReferenceName(name), resolved)
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func (c NativeGit) Push(remoteName string, refspecs []string) error {
	remote, err := c.repo().Remote(remoteName)
	if err != nil {
		return fmt.Errorf("getting remote %s %w", remoteName, err)
	}

	gogitrefspecs := []gogitconfig.RefSpec{}
	for _, refspec := range refspecs {
		gogitrefspecs = append(gogitrefspecs, gogitconfig.RefSpec(refspec))
	}

	pushOptions := gogit.PushOptions{
		RemoteName: remoteName,
		// Nothing before the colon says to push nothing to the destination branch (which deletes it).
		RefSpecs: gogitrefspecs,
	}

	err = remote.Push(&pushOptions)
	if err != nil {
		return fmt.Errorf("pushing %w", err)
	}

	return nil
}

// RemoteBranches returns a list of all remote branches
func (c NativeGit) RemoteBranches() (mapset.Set[string], error) {
	remoteBranches := mapset.NewSet[string]()
	remote, err := c.repo().Remote(c.base.Config.Repo.GitHubRemote)
	if err != nil {
		return remoteBranches, fmt.Errorf("finding remote branches %w", err)
	}

	refs, err := remote.List(&gogit.ListOptions{})
	if err != nil {
		return remoteBranches, fmt.Errorf("listing remote branches %w", err)
	}
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			remoteBranches.Add(ref.Name().String())
		}
	}
	return remoteBranches, nil
}

func (c NativeGit) BranchExists(branchName string) (bool, error) {
	iter, err := c.repo().Branches()
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

// OriginMainRef returns the ref for the default remote and the default branch (often origin/main)
func (c NativeGit) OriginMainRef(ctx context.Context) (string, error) {
	branch := c.base.Config.Repo.GitHubBranch

	return c.OriginBranchRef(ctx, branch)
}

// OriginBranchRef returns the ref for the default remote (often origin) and the given branch
func (c NativeGit) OriginBranchRef(ctx context.Context, branch string) (string, error) {
	remote := c.base.Config.Repo.GitHubRemote

	originMainRef, err := c.Reference(fmt.Sprintf("refs/remotes/%s/%s", remote, branch), true)
	if err != nil {
		return "", fmt.Errorf("getting %s/%s HEAD %w", remote, branch, err)
	}

	return originMainRef, nil
}

func (c NativeGit) UnMergedCommits(ctx context.Context) ([]*object.Commit, error) {
	headRef, err := c.repo().Head()
	if err != nil {
		return nil, fmt.Errorf("getting repo HEAD %w", err)
	}

	commitIter, err := c.repo().Log(&gogit.LogOptions{From: headRef.Hash()})
	if err != nil {
		return nil, fmt.Errorf("getting iterator for commits %w", err)
	}

	originMainRefName, err := c.OriginMainRef(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting origin main ref %w", err)
	}

	commits := []*object.Commit{}

	commitIter.ForEach(func(cm *object.Commit) error {
		if originMainRefName == cm.Hash.String() {
			return storer.ErrStop
		}
		commits = append(commits, cm)
		return nil
	})

	return commits, nil
}
