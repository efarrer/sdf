package mockgit

import (
	"context"
	"fmt"
	"strings"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/mock"
)

// NewMockGit creates and new mock git instance
func NewMockGit(expectations *mock.Expectations) *Mock {
	return &Mock{
		expectations: expectations,
	}
}

func (m *Mock) GitWithEditor(args string, output *string, editorCmd string) error {
	return m.Git(args, output)
}

func (m *Mock) Git(args string, output *string) error {
	m.expectations.GitCmd("git "+args, output)
	return nil
}

func (m *Mock) DeleteRemoteBranch(ctx context.Context, branch string) error {
	return m.Git(fmt.Sprintf("DeleteRemoteBranch(%s)", branch), nil)
}

func (m *Mock) GetLocalBranchShortName() (string, error) {
	return "", m.Git(fmt.Sprintf("GetLocalBranchShortName()"), nil)
}

func (m *Mock) ExpectationsMet() {
	m.expectations.ExpectationsMet()
}

func (m *Mock) MustGit(argStr string, output *string) {
	err := m.Git(argStr, output)
	if err != nil {
		panic(err)
	}
}

func (m *Mock) RootDir() string {
	return ""
}

type Mock struct {
	expectations *mock.Expectations
}

func (m *Mock) ExpectFetch() {
	m.expect("git fetch")
	m.expect("git rebase origin/master --autostash")
}

func (m *Mock) ExpectDeleteBranch(branchName string) {
	m.expect(fmt.Sprintf("git DeleteRemoteBranch(%s)", branchName))
}

func (m *Mock) ExpectGetLocalBranchShortName() {
	m.expect(fmt.Sprintf("git GetLocalBranchShortName()"))
}

func (m *Mock) ExpectLogAndRespond(commits []*git.Commit) {
	m.expect("git log --format=medium --no-color origin/master..HEAD", mock.CommitOutputter(commits))
}

func (m *Mock) ExpectStatus() {
	m.expect("git status --porcelain --untracked-files=no")
}

func (m *Mock) ExpectPushCommits(commits []*git.Commit) {
	m.ExpectStatus()

	var refNames []string
	for _, c := range commits {
		branchName := "spr/master/" + c.CommitID
		refNames = append(refNames, c.CommitHash+":refs/heads/"+branchName)
	}
	m.expect("git push --force --atomic origin " + strings.Join(refNames, " "))
}

func (m *Mock) ExpectRemote(remote string) {
	response := fmt.Sprintf("origin  %s (fetch)\n", remote)
	response += fmt.Sprintf("origin  %s (push)\n", remote)
	m.expect("git remote -v", mock.StringOutputter(response))
}

func (m *Mock) ExpectFixup(commitHash string) {
	m.expect("git commit --fixup " + commitHash)
	m.expect("git rebase -i --autosquash --autostash origin/master")
}

func (m *Mock) ExpectLocalBranch(name string) {
	m.expect("git branch --no-color", mock.StringOutputter(name))
}

func (m *Mock) expect(cmd string, response ...mock.Outputter) {
	m.expectations.ExpectGit(cmd, response...)
}
