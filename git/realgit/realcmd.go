package realgit

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	gitconfig "github.com/go-git/go-git/v5/config"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git/realgit/internal"
)

// NewGitCmd returns a new git cmd instance
func NewGitCmd(cfg *config.Config) *gitcmd {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	wt, err := internal.Repo().Worktree()
	if err != nil {
		fmt.Printf("%s is a bare git repository\n", cwd)
		os.Exit(2)
	}

	rootdir := wt.Filesystem.Root()
	rootdir = strings.TrimSpace(maybeAdjustPathPerPlatform(rootdir))

	base := &internal.Gitbase{
		Config:  cfg,
		Rootdir: rootdir,
		Stderr:  os.Stderr,
	}
	return &gitcmd{
		NativeGit: internal.NewNativeGit(base),
		CmdLine:   internal.NewCmdLine(base),
		base:      base,
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
	internal.NativeGit
	internal.CmdLine
	base *internal.Gitbase
}

func (c *gitcmd) RootDir() string {
	return c.base.Rootdir
}

func (c *gitcmd) SetRootDir(newroot string) {
	c.base.Rootdir = newroot
}

func (c *gitcmd) SetStderr(stderr io.Writer) {
	c.base.Stderr = stderr
}

func (c *gitcmd) Email() (string, error) {
	cfg, err := gitconfig.LoadConfig(gitconfig.GlobalScope)
	if err != nil {
		return "", fmt.Errorf("getting user email %w", err)
	}

	return cfg.User.Email, nil
}
