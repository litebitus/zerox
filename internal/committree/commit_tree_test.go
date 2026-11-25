package committree

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/litebitus/zerox/internal/repositorytest"
)

func TestTheTemplatesAreReadAsTheCommitHoldsThem(testingContext *testing.T) {
	testRepository := repositorytest.New(testingContext)
	testRepository.WriteFile("zerox/mock-set/zerox.yml", "committed\n")
	testRepository.WriteFile("zerox/mock-set/run.sh", "echo mock\n")
	testRepository.WriteFile("elsewhere.txt", "outside\n")
	if err := os.Chmod(filepath.Join(testRepository.Root, "zerox", "mock-set", "run.sh"), 0o755); err != nil {
		testingContext.Fatal(err)
	}
	commit := testRepository.CommitAll("templates")
	testRepository.WriteFile("zerox/mock-set/zerox.yml", "uncommitted\n")

	repository, err := Open(filepath.Join(testRepository.Root, "zerox"))
	if err != nil {
		testingContext.Fatal(err)
	}
	files, err := repository.TemplatesAt(commit)
	if err != nil {
		testingContext.Fatal(err)
	}
	content, err := fs.ReadFile(files, "zerox/mock-set/zerox.yml")
	if err != nil || string(content) != "committed\n" {
		testingContext.Fatalf("zerox.yml %q: %v", content, err)
	}
	if info, err := fs.Stat(files, "zerox/mock-set/run.sh"); err != nil || info.Mode()&0o111 == 0 {
		testingContext.Fatalf("run.sh %v: %v", info, err)
	}
	if _, err := fs.Stat(files, "elsewhere.txt"); err == nil {
		testingContext.Fatal("a file outside the templates folder was read")
	}
}

func TestARefNamesACommitAndAVersionReadsItsReleaseTag(testingContext *testing.T) {
	testRepository := repositorytest.New(testingContext)
	testRepository.WriteFile("zerox/mock-set/zerox.yml", "mock\n")
	commit := testRepository.CommitAll("templates")
	testRepository.Git("tag", "--annotate", "zerox-templates/v1.2.3", "--message", "mock release")
	testRepository.Git("tag", "zerox-templates/v1")
	repository, err := Open(testRepository.Root)
	if err != nil {
		testingContext.Fatal(err)
	}
	shortCommit, err := repository.ShortCommit(commit)
	if err != nil || !strings.HasPrefix(commit, shortCommit) {
		testingContext.Fatalf("short commit %q: %v", shortCommit, err)
	}
	for _, ref := range []string{"HEAD", "v1.2.3", "v1", shortCommit} {
		if resolved, err := repository.Commit(ref); err != nil || resolved != commit {
			testingContext.Fatalf("%s resolved to %q: %v", ref, resolved, err)
		}
	}
	if _, err := repository.Commit("v2"); err == nil || err.Error() != "the repository has no tag `zerox-templates/v2`" {
		testingContext.Fatalf("v2: %v", err)
	}
	if _, err := repository.Commit("abcdef0"); err == nil || err.Error() != "`abcdef0` names no commit in the repository" {
		testingContext.Fatalf("abcdef0: %v", err)
	}
	if _, err := repository.ShortCommit("abcdef0"); err == nil {
		testingContext.Fatal("a missing commit was abbreviated")
	}
}

func TestACommitWithoutTemplatesOrWithALinkInThemIsRefused(testingContext *testing.T) {
	testRepository := repositorytest.New(testingContext)
	testRepository.WriteFile("readme.txt", "mock\n")
	bare := testRepository.CommitAll("no templates")
	if err := os.MkdirAll(filepath.Join(testRepository.Root, "zerox"), 0o755); err != nil {
		testingContext.Fatal(err)
	}
	if err := os.Symlink("../readme.txt", filepath.Join(testRepository.Root, "zerox", "link.txt")); err != nil {
		testingContext.Fatal(err)
	}
	linked := testRepository.CommitAll("a link")
	repository := Repository{Root: testRepository.Root}
	if _, err := repository.TemplatesAt(bare); err == nil || !strings.Contains(err.Error(), "holds no `zerox` folder") {
		testingContext.Fatalf("bare: %v", err)
	}
	if _, err := repository.TemplatesAt(linked); err == nil || !strings.Contains(err.Error(), "zerox/link.txt of commit") {
		testingContext.Fatalf("linked: %v", err)
	}
}

func TestAFolderOutsideARepositoryIsRefused(testingContext *testing.T) {
	if _, err := Open(testingContext.TempDir()); err == nil || !strings.Contains(err.Error(), "is not in a git repository's working tree") {
		testingContext.Fatalf("%v", err)
	}
}

func TestOutputPastItsLimitFailsTheCommand(testingContext *testing.T) {
	limited := &limitedBuffer{buffer: &bytes.Buffer{}, limit: 4}
	if _, err := limited.Write([]byte("1234")); err != nil {
		testingContext.Fatal(err)
	}
	if _, err := limited.Write([]byte("5")); err == nil || limited.buffer.String() != "1234" {
		testingContext.Fatalf("output past the limit was held: %q %v", limited.buffer.String(), err)
	}
}
