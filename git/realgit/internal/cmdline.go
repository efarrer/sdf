package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

type CmdLine struct {
	base *Gitbase
}

func NewCmdLine(base *Gitbase) CmdLine {
	return CmdLine{base: base}
}

func (c CmdLine) Git(argStr string, output *string) error {
	return c.GitWithEditor(argStr, output, "/usr/bin/true")
}

func (c CmdLine) MustGit(argStr string, output *string) {
	err := c.Git(argStr, output)
	if err != nil {
		panic(err)
	}
}

func (c CmdLine) GitWithEditor(argStr string, output *string, editorCmd string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command

	// Rebase disabled
	_, noRebaseFlag := os.LookupEnv("SPR_NOREBASE")
	if (c.base.Config.User.NoRebase || noRebaseFlag) && strings.HasPrefix(argStr, "rebase") {
		return nil
	}

	log.Debug().Msg("git " + argStr)
	if c.base.Config.User.LogGitCommands {
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
	cmd.Dir = c.base.Rootdir

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
			fmt.Fprintf(c.base.Stderr, "git error: %s", string(out))
			return err
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(c.base.Stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
}

func (c CmdLine) AppendCommitId() error {
	rewordPath, err := exec.LookPath("spr_reword_helper")
	if err != nil {
		fmt.Errorf("can't find spr_reword_helper %w", err)
	}
	rebaseCommand := fmt.Sprintf(
		"rebase %s/%s -i --autosquash --autostash",
		c.base.Config.Repo.GitHubRemote,
		c.base.Config.Repo.GitHubBranch,
	)
	err = c.GitWithEditor(rebaseCommand, nil, rewordPath)
	if err != nil {
		fmt.Errorf("can't execute spr_reword_helper %w", err)
	}

	return nil
}

func (c *CmdLine) Rebase(ctx context.Context, remoteName, branchName string) error {
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
