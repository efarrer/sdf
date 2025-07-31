package mockoutput

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/ejoffe/spr/output"
)

type matcher string

func (str matcher) Match(s string) bool {
	return string(str) == s
}

type rematcher string

func (re rematcher) Match(s string) bool {
	matched, err := regexp.MatchString(string(re), s)
	if err != nil {
		panic(err.Error())
	}

	return matched
}

// CapturedOutput is a printer spy for testing
type CapturedOutput struct {
	printed  []string
	expected []output.Matcher
	lock     *sync.Mutex
}

func (co *CapturedOutput) Print(str string) {
	co.lock.Lock()
	defer co.lock.Unlock()
	co.printed = append(co.printed, str)
}

func (co *CapturedOutput) Printf(format string, a ...any) output.Printer {
	co.lock.Lock()
	defer co.lock.Unlock()
	co.printed = append(co.printed, fmt.Sprintf(format, a...))
	return co
}

func (co *CapturedOutput) Expect(matcher output.Matcher) *CapturedOutput {
	co.lock.Lock()
	defer co.lock.Unlock()
	co.expected = append(co.expected, matcher)

	return co
}

func (co *CapturedOutput) ExpectString(str string) *CapturedOutput {
	return co.Expect(matcher(str))
}

func (co *CapturedOutput) ExpectRegExp(str string) *CapturedOutput {
	co.lock.Lock()
	defer co.lock.Unlock()
	co.expected = append(co.expected, rematcher(str))

	return co
}

func (co *CapturedOutput) Purge() {
	co.lock.Lock()
	defer co.lock.Unlock()

	co.printed = []string{}
	co.expected = []output.Matcher{}
}

func (co *CapturedOutput) ExpectationsMet() {
	co.lock.Lock()
	defer co.lock.Unlock()

	if len(co.printed) != len(co.expected) {
		co.fail(nil)
		return
	}
	for i := 0; i != len(co.printed); i++ {
		if !co.expected[i].Match(co.printed[i]) {
			co.fail(&i)
			return
		}
	}

	co.printed = []string{}
	co.expected = []output.Matcher{}
}

func (co *CapturedOutput) fail(index *int) {
	msg := "Expected:\n"
	for i := 0; i < len(co.expected); i++ {
		got := co.expected[i]
		if index != nil && *index == i {
			msg += fmt.Sprintf("-----> \"%s\"\n", got)
		} else {
			msg += fmt.Sprintf("\"%s\"\n", got)
		}
	}

	msg += "Got:\n"
	for i := 0; i < len(co.printed); i++ {
		got := co.printed[i]
		if index != nil && *index == i {
			msg += fmt.Sprintf("-----> \"%s\"\n", got)
		} else {
			msg += fmt.Sprintf("\"%s\"\n", got)
		}
	}

	msg += "instead\n"

	panic(msg)
}

func MockPrinter() *CapturedOutput {
	return &CapturedOutput{
		printed: []string{},
		lock:    &sync.Mutex{},
	}
}
