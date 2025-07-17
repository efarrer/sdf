package mock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectations(t *testing.T) {
	t.Run("git", func(t *testing.T) {
		t.Run("matches single expectation", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGit("git status")

			out, err := m.check(GitExpectation{command: "git status"})
			require.Nil(t, out)
			require.NoError(t, err)
		})

		t.Run("matches multiple expectations", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGit("git status")
			m.ExpectGit("git log")

			out, err := m.check(GitExpectation{command: "git status"})
			require.Nil(t, out)
			require.NoError(t, err)
			out, err = m.check(GitExpectation{command: "git log"})
			require.Nil(t, out)
			require.NoError(t, err)
		})

		t.Run("fails if no expectations set", func(t *testing.T) {
			m := New(t, true)

			out, err := m.check(GitExpectation{command: "git status"})
			require.Nil(t, out)
			require.Error(t, err)
		})

		t.Run("fails if not enough expectations set", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGit("git status")

			out, err := m.check(GitExpectation{command: "git status"})
			require.Nil(t, out)
			require.NoError(t, err)
			out, err = m.check(GitExpectation{command: "git status"})
			require.Nil(t, out)
			require.Error(t, err)
		})

		t.Run("fails if bad expectations set", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGit("git status")

			out, err := m.check(GitExpectation{command: "git log"})
			require.Nil(t, out)
			require.Error(t, err)
		})

		t.Run("matches with output", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGit("git status", StringOutputter("some output"))

			out, err := m.check(GitExpectation{command: "git status"})
			require.Equal(t, *out, "some output")
			require.NoError(t, err)
		})
	})

	t.Run("github", func(t *testing.T) {
		t.Run("matches single expectation", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})

			out, err := m.check(GithubExpectation{Op: GetInfoOP})
			require.Nil(t, out)
			require.NoError(t, err)
		})

		t.Run("matches multiple expectations", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})
			m.ExpectGitHub(GithubExpectation{Op: GetAssignableUsersOP})

			out, err := m.check(GithubExpectation{Op: GetInfoOP})
			require.Nil(t, out)
			require.NoError(t, err)
			out, err = m.check(GithubExpectation{Op: GetAssignableUsersOP})
			require.Nil(t, out)
			require.NoError(t, err)
		})

		t.Run("fails if no expectations set", func(t *testing.T) {
			m := New(t, true)

			out, err := m.check(GithubExpectation{Op: GetInfoOP})
			require.Nil(t, out)
			require.Error(t, err)
		})

		t.Run("fails if not enough expectations set", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})

			out, err := m.check(GithubExpectation{Op: GetInfoOP})
			require.Nil(t, out)
			require.NoError(t, err)
			out, err = m.check(GithubExpectation{Op: GetAssignableUsersOP})
			require.Nil(t, out)
			require.Error(t, err)
		})

		t.Run("fails if bad expectations set", func(t *testing.T) {
			m := New(t, true)

			m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})

			out, err := m.check(GithubExpectation{Op: GetAssignableUsersOP})
			require.Nil(t, out)
			require.Error(t, err)
		})
	})

	t.Run("mixed expectation types", func(t *testing.T) {
		m := New(t, true)

		m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})
		m.ExpectGit("git status")
		m.ExpectGitHub(GithubExpectation{Op: GetInfoOP})

		out, err := m.check(GithubExpectation{Op: GetInfoOP})
		require.Nil(t, out)
		require.NoError(t, err)
		out, err = m.check(GitExpectation{command: "git status"})
		require.Nil(t, out)
		require.NoError(t, err)
		out, err = m.check(GitExpectation{command: "git status"})
		require.Nil(t, out)
		require.Error(t, err)
	})

	t.Run("matches unsynchronized", func(t *testing.T) {
		m := New(t, false)

		m.ExpectGit("git log")
		m.ExpectGit("git status")

		out, err := m.check(GitExpectation{command: "git status"})
		require.Nil(t, out)
		require.NoError(t, err)
		out, err = m.check(GitExpectation{command: "git log"})
		require.Nil(t, out)
		require.NoError(t, err)
	})
}
