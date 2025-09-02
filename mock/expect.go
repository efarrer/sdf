package mock

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/githubclient/genqlient"
)

type Outputter interface {
	Output() *string
}

type Operation interface {
	fmt.Stringer
	Outputter
}

type NilOutputter int

func (NilOutputter) Output() *string {
	return nil
}

func (NilOutputter) String() string {
	return ""
}

type StringOutputter string

func (so StringOutputter) Output() *string {
	val := string(so)
	return &val
}

type CommitOutputter []*git.Commit

func (co CommitOutputter) Output() *string {
	r := []*git.Commit(co)
	var b strings.Builder
	for _, c := range r {
		fmt.Fprintf(&b, "commit %s\n", c.CommitHash)
		fmt.Fprintf(&b, "Author: Eitan Joffe <ejoffe@gmail.com>\n")
		fmt.Fprintf(&b, "Date:   Fri Jun 11 14:15:49 2021 -0700\n")
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "\t%s\n", c.Subject)
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "\tcommit-id:%s\n", c.CommitID)
		fmt.Fprintf(&b, "\n")
	}

	val := b.String()
	return &val
}

type operation string

const (
	GetInfoOP                   = "GetInfo"
	GetAssignableUsersOP        = "GetAssignableUsers"
	CreatePullRequestOP         = "CreatePullRequest"
	UpdatePullRequestOP         = "UpdatePullRequest"
	AddReviewersOP              = "AddReviewers"
	CommentPullRequestOP        = "CommentPullRequest"
	MergePullRequestOP          = "MergePullRequest"
	ClosePullRequestOP          = "ClosePullRequest"
	ClosePullRequestAndStatusOP = "ClosePullRequestAndStatus"
	EditPullRequestOP           = "EditPullRequest"
	ListPullRequestsOP          = "ListPullRequests"
	GetPullRequestOP            = "GetPullRequest"
	ListPullRequestReviewsOP    = "ListPullRequestReviews"
	GetCombinedStatusOP         = "GetCombinedStatus"
)

type GithubExpectation struct {
	Op          operation
	Commit      git.Commit
	Prev        *git.Commit
	MergeMethod genqlient.PullRequestMergeMethod
	UserIDs     []string
}

func (ge GithubExpectation) String() string {
	data, err := json.Marshal(ge)
	if err != nil {
		panic(err.Error())
	}
	return string(data)
}

func (ge GithubExpectation) Output() *string {
	return nil
}

type GitExpectation struct {
	command string
	output  Outputter
}

func (ge GitExpectation) String() string {
	return ge.command
}

func (ge GitExpectation) Output() *string {
	if ge.output == nil {
		return nil
	}
	return ge.output.Output()
}

type Expectations struct {
	t                    *testing.T
	expectations         []Operation
	realities            []Operation
	nextExpectationIndex int
	mu                   *sync.Mutex
	synchronized         bool
}

func New(t *testing.T, synchronized bool) *Expectations {
	return &Expectations{
		t:            t,
		mu:           &sync.Mutex{},
		synchronized: synchronized,
	}
}

func (e *Expectations) ExpectationsMet() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.synchronized {
		if e.nextExpectationIndex != len(e.expectations) {
			e.fail(fmt.Sprintf("expected the additional commands: %v", e.expectations[e.nextExpectationIndex:]))
		}
	} else {
		for i := 0; i != len(e.expectations); i++ {
			if _, ok := e.expectations[i].(NilOutputter); !ok {
				e.fail(fmt.Sprintf("expected the additional command: %v", e.expectations[i]))
			}
		}
	}

	// Clear out the existing expectations since they were met
	e.nextExpectationIndex = 0
	e.expectations = []Operation{}
	e.realities = []Operation{}
}

func (e *Expectations) ExpectGit(cmd string, response ...Outputter) {
	e.mu.Lock()
	defer e.mu.Unlock()

	exp := GitExpectation{command: cmd}
	if len(response) > 0 {
		exp.output = response[0]
	}
	e.expectations = append(e.expectations, exp)
}

func (e *Expectations) GitCmd(cmd string, output *string) {
	out, err := e.check(GitExpectation{command: cmd})
	if err != nil {
		e.fail(err.Error())
	}
	if out != nil {
		*output = *out
	}
}

func (e *Expectations) check(cmd Operation) (*string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.realities = append(e.realities, cmd)

	if e.synchronized {
		if len(e.expectations) == 0 {
			return nil, fmt.Errorf("Unexpected command when no expectations were set: \"%s\"", cmd)
		}

		if e.nextExpectationIndex >= len(e.expectations) {
			return nil, fmt.Errorf("Unexpected command:\n\"%s\"\n The previous executed command was:\n\"%s\"", cmd, e.expectations[e.nextExpectationIndex-1])
		}

		exp := e.expectations[e.nextExpectationIndex]
		if exp.String() != cmd.String() {
			msg := "Expected:\n"
			for i := 0; i < e.nextExpectationIndex; i++ {
				got := e.expectations[i]
				msg += fmt.Sprintf("\"%s\"\n", got)
			}
			msg += fmt.Sprintf("-----> \"%s\"\n", exp.String())

			msg += "Got:\n"
			for i := 0; i < len(e.realities)-1; i++ {
				got := e.realities[i]
				msg += fmt.Sprintf("\"%s\"\n", got)
			}
			msg += fmt.Sprintf("-----> \"%s\"\n", cmd.String())

			msg += "instead\n"

			return nil, errors.New(msg)
		}

		e.nextExpectationIndex++
		return exp.Output(), nil
	} else {
		for i := 0; i != len(e.expectations); i++ {
			if e.expectations[i].String() == cmd.String() {
				exp := e.expectations[i]
				e.expectations[i] = NilOutputter(0)
				return exp.Output(), nil
			}
		}
		return nil, fmt.Errorf("Unexpected command:\n\"%s\"\n", cmd)
	}
}

func (e *Expectations) ExpectGitHub(exp GithubExpectation) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.expectations = append(e.expectations, exp)
}

func (e *Expectations) GithubApi(cmd GithubExpectation) {
	_, err := e.check(cmd)
	if err != nil {
		e.fail(err.Error())
	}
}

func (e *Expectations) fail(msg string) {
	fmt.Println("-------------------------- BEGIN FAILED --------------------------")
	fmt.Printf("Test: %s failed\n", e.t.Name())
	fmt.Printf("%s\n", msg)
	fmt.Println("--------------------------  END FAILED --------------------------")
	panic("")
}
