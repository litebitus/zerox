// Package repositorytest makes git repositories in a test's temporary folder, isolated from the machine's git config.
package repositorytest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepository is a git repository at a test's temporary folder.
type TestRepository struct {
	Root    string
	testing *testing.T
}

// New is an empty git repository whose branch is `main`.
func New(testingContext *testing.T) TestRepository {
	testingContext.Helper()
	repository := TestRepository{Root: testingContext.TempDir(), testing: testingContext}
	repository.Git("init", "--quiet", "--initial-branch=main")
	return repository
}

// Git runs git in the repository and is its output, failing the test when git fails.
func (repository TestRepository) Git(arguments ...string) string {
	repository.testing.Helper()
	command := exec.Command("git", append([]string{"-C", repository.Root, "-c", "user.name=mock-user", "-c", "user.email=mock-user@example.com",
		"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		repository.testing.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// WriteFile writes a file at a path from the repository's root, making its folders.
func (repository TestRepository) WriteFile(filePath, content string) {
	repository.testing.Helper()
	absolute := filepath.Join(repository.Root, filepath.FromSlash(filePath))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		repository.testing.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		repository.testing.Fatal(err)
	}
}

// ReadFile is the content of a file at a path from the repository's root.
func (repository TestRepository) ReadFile(filePath string) string {
	repository.testing.Helper()
	content, err := os.ReadFile(filepath.Join(repository.Root, filepath.FromSlash(filePath)))
	if err != nil {
		repository.testing.Fatal(err)
	}
	return string(content)
}

// CommitAll commits every change in the working tree and is the commit.
func (repository TestRepository) CommitAll(message string) string {
	repository.testing.Helper()
	repository.Git("add", "--all")
	repository.Git("commit", "--quiet", "--allow-empty", "--message", message)
	return repository.Git("rev-parse", "HEAD")
}
