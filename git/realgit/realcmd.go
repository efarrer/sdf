package realgit

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	gitconfig "github.com/go-git/go-git/v5/config"

	"github.com/ejoffe/spr/config"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gogitplumbing "github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/rs/zerolog/log"
)

// repo creates a *gogit.Repository the *gogit.Repository should not be shared between goroutines
func repo() *gogit.Repository {
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

// NewGitCmd returns a new git cmd instance
func NewGitCmd(cfg *config.Config) *gitcmd {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	wt, err := repo().Worktree()
	if err != nil {
		fmt.Printf("%s is a bare git repository\n", cwd)
		os.Exit(2)
	}

	rootdir := wt.Filesystem.Root()
	rootdir = strings.TrimSpace(maybeAdjustPathPerPlatform(rootdir))

	return &gitcmd{
		config:  cfg,
		rootdir: rootdir,
		stderr:  os.Stderr,
	}
}

func maybeAdjustPathPerPlatform(rawRootDir string) string {
	if strings.HasPrefix(rawRootDir, "/cygdrive") {
		// This is safe to run also on "proper" Windows paths
		cmd := exec.Command("cygpath", []string{"-w", rawRootDir}...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}
		return string(out)
	}

	return rawRootDir
}

type gitcmd struct {
	config  *config.Config
	rootdir string
	stderr  io.Writer
}

func (c *gitcmd) repo() *gogit.Repository {
	return repo()
}

func (c *gitcmd) Git(argStr string, output *string) error {
	return c.GitWithEditor(argStr, output, "/usr/bin/true")
}

func (c *gitcmd) MustGit(argStr string, output *string) {
	err := c.Git(argStr, output)
	if err != nil {
		panic(err)
	}
}

func (c gitcmd) AppendCommitId() error {
	rewordPath, err := exec.LookPath("spr_reword_helper")
	if err != nil {
		fmt.Errorf("can't find spr_reword_helper %w", err)
	}
	rebaseCommand := fmt.Sprintf(
		"rebase %s/%s -i --autosquash --autostash",
		c.config.Repo.GitHubRemote,
		c.config.Repo.GitHubBranch,
	)
	err = c.GitWithEditor(rebaseCommand, nil, rewordPath)
	if err != nil {
		fmt.Errorf("can't execute spr_reword_helper %w", err)
	}

	return nil
}

func (c *gitcmd) GitWithEditor(argStr string, output *string, editorCmd string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command

	// Rebase disabled
	_, noRebaseFlag := os.LookupEnv("SPR_NOREBASE")
	if (c.config.User.NoRebase || noRebaseFlag) && strings.HasPrefix(argStr, "rebase") {
		return nil
	}

	log.Debug().Msg("git " + argStr)
	if c.config.User.LogGitCommands {
		fmt.Printf("> git %s\n", argStr)
	}
	args := []string{
		"-c", fmt.Sprintf("core.editor=%s", editorCmd),
		"-c", "commit.verbose=false",
		"-c", "rebase.abbreviateCommands=false",
		"-c", fmt.Sprintf("sequence.editor=%s", editorCmd),
	}
	args = append(args, strings.Split(argStr, " ")...)
	cmd := exec.Command("git", args...)
	cmd.Dir = c.rootdir

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)

		if parts[1] != "" && strings.ToUpper(parts[0]) != "EDITOR" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", parts[0], parts[1]))
		}
	}

	if output != nil {
		out, err := cmd.CombinedOutput()
		*output = strings.TrimSpace(string(out))
		if err != nil {
			fmt.Fprintf(c.stderr, "git error: %s", string(out))
			return err
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(c.stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
}

func (c *gitcmd) RootDir() string {
	return c.rootdir
}

func (c *gitcmd) SetRootDir(newroot string) {
	c.rootdir = newroot
}

func (c *gitcmd) SetStderr(stderr io.Writer) {
	c.stderr = stderr
}

func (c *gitcmd) DeleteRemoteBranch(ctx context.Context, branch string) error {
	remoteName := c.config.Repo.GitHubRemote

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
func (c *gitcmd) GetLocalBranchShortName() (string, error) {
	ref, err := c.repo().Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD %w", err)
	}

	return string(ref.Name().Short()), nil
}

// Fetch fetches references along with the objects necessary to complete
// their histories, from the remote named as FetchOptions.RemoteName.
func (c *gitcmd) Fetch(remoteName string, prune bool) error {
	return c.repo().Fetch(&gogit.FetchOptions{
		RemoteName: remoteName,
		Prune:      prune,
	})
}

// Reference returns the reference for a given reference name. If resolved is
// true, any symbolic reference will be resolved.
func (c *gitcmd) Reference(name string, resolved bool) (string, error) {
	ref, err := c.repo().Reference(plumbing.ReferenceName(name), resolved)
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func (c *gitcmd) Push(remoteName string, refspecs []string) error {
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
func (c *gitcmd) RemoteBranches() (mapset.Set[string], error) {
	remoteBranches := mapset.NewSet[string]()
	remote, err := c.repo().Remote(c.config.Repo.GitHubRemote)
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

func (c *gitcmd) BranchExists(branchName string) (bool, error) {
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
func (c *gitcmd) OriginMainRef(ctx context.Context) (string, error) {
	branch := c.config.Repo.GitHubBranch

	return c.OriginBranchRef(ctx, branch)
}

// OriginBranchRef returns the ref for the default remote (often origin) and the given branch
func (c *gitcmd) OriginBranchRef(ctx context.Context, branch string) (string, error) {
	remote := c.config.Repo.GitHubRemote

	originMainRef, err := c.Reference(fmt.Sprintf("refs/remotes/%s/%s", remote, branch), true)
	if err != nil {
		return "", fmt.Errorf("getting %s/%s HEAD %w", remote, branch, err)
	}

	return originMainRef, nil
}

func (c *gitcmd) UnMergedCommits(ctx context.Context) ([]*object.Commit, error) {
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

func (c *gitcmd) Rebase(ctx context.Context, remoteName, branchName string) error {
	err := c.Git(
		fmt.Sprintf("rebase %s/%s -i --autosquash --autostash",
			remoteName,
			branchName,
		), nil)
	if err != nil {
		return fmt.Errorf("rebase failed %w", err)
	}

	return nil
}

func (c *gitcmd) Email() (string, error) {
	cfg, err := gitconfig.LoadConfig(gitconfig.GlobalScope)
	if err != nil {
		return "", fmt.Errorf("getting user email %w", err)
	}

	return cfg.User.Email, nil
}
