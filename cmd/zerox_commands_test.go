package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/litebitus/zerox/internal/repositorytest"
	"github.com/litebitus/zerox/internal/targetfiles"
)

const commonConfig = `version: "1"
delimiters: {left: "[[", right: "]]"}
default_tags: [operator]
inputs:
  type: object
  properties:
    pinned:
      type: string
      default: "1.0.2"
`

const setConfig = `version: "1"
target: mocks/[[ .mock_id ]]
inputs:
  type: object
  additionalProperties: false
  required: [mock_id]
  properties:
    mock_id:
      type: string
      pattern: "^m[0-9]+$"
      tags: [requester]
    limits:
      type: object
      properties:
        size: {type: integer, default: 10}
`

var mockTime = time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)

type commandRun struct {
	exitCode    int
	output      string
	errorOutput string
}

func runIn(testingContext *testing.T, workingFolder string, arguments ...string) commandRun {
	testingContext.Helper()
	var output, errorOutput bytes.Buffer
	exitCode := Run("mock-version", arguments, workingFolder, &output, &errorOutput, func() time.Time { return mockTime })
	return commandRun{exitCode: exitCode, output: output.String(), errorOutput: errorOutput.String()}
}

func templatesRepository(testingContext *testing.T) repositorytest.TestRepository {
	testingContext.Helper()
	repository := repositorytest.New(testingContext)
	repository.WriteFile("zerox/zerox.yml", commonConfig)
	repository.WriteFile("zerox/mock-set/zerox.yml", setConfig)
	repository.WriteFile("zerox/mock-set/mock.yml", "id: [[ .mock_id ]]\nsize: [[ .limits.size ]]\npinned: [[ .pinned ]]\nkeep: {{MOCK_PLACEHOLDER}}\n")
	repository.WriteFile("zerox/mock-set/sub/gone.txt", "gone [[ .mock_id ]]\n")
	repository.CommitAll("templates")
	return repository
}

func expectRun(testingContext *testing.T, actual commandRun, exitCode int, outputLines ...string) {
	testingContext.Helper()
	if actual.exitCode != exitCode {
		testingContext.Fatalf("exit code %d, expected %d\noutput:\n%s\nerrors:\n%s", actual.exitCode, exitCode, actual.output, actual.errorOutput)
	}
	if expected := strings.Join(outputLines, "\n") + "\n"; len(outputLines) > 0 && actual.output != expected {
		testingContext.Fatalf("output:\n%s\nexpected:\n%s", actual.output, expected)
	}
}

func TestGenerateRendersANewTargetAndRecordsItsValuesLineageTagsAndFingerprints(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	shortCommit := repository.Git("rev-parse", "--short", "HEAD")
	expectRun(testingContext, runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1"), 0,
		"written mocks/m1/mock.yml",
		"written mocks/m1/sub/gone.txt",
		"written mocks/m1/zerox.yml")
	if actual := repository.ReadFile("mocks/m1/mock.yml"); actual != "id: m1\nsize: 10\npinned: 1.0.2\nkeep: {{MOCK_PLACEHOLDER}}\n" {
		testingContext.Fatalf("mock.yml:\n%s", actual)
	}
	expectedConfig := `version: "1"
template: mock-set@` + shortCommit + `
generated_at: "2026-11-25T08:00:00Z"
values:
  limits:
    size: 10
  mock_id: m1
  pinned: 1.0.2
input_tags:
  limits:
    - operator
  limits.size:
    - operator
  mock_id:
    - requester
  pinned:
    - operator
fingerprints:
  mock.yml: ` + fingerprintOf("id: m1\nsize: 10\npinned: 1.0.2\nkeep: {{MOCK_PLACEHOLDER}}\n") + `
  sub/gone.txt: ` + fingerprintOf("gone m1\n") + `
`
	if actual := repository.ReadFile("mocks/m1/zerox.yml"); actual != expectedConfig {
		testingContext.Fatalf("zerox.yml:\n%s\nexpected:\n%s", actual, expectedConfig)
	}
}

func TestARetriedGenerateIsANoOpAndOneWithOtherValuesIsRefused(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1")
	before := repository.ReadFile("mocks/m1/zerox.yml")
	expectRun(testingContext, runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1"), 0,
		"unchanged mocks/m1/mock.yml",
		"unchanged mocks/m1/sub/gone.txt",
		"unchanged mocks/m1/zerox.yml")
	if repository.ReadFile("mocks/m1/zerox.yml") != before {
		testingContext.Fatal("a retried generate rewrote zerox.yml")
	}
	refused := runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1", "--value", "limits.size=3")
	expectRun(testingContext, refused, 1)
	if !strings.Contains(refused.errorOutput, "`mocks/m1` was generated with other values; `zerox update mocks/m1` changes them") {
		testingContext.Fatalf("errors:\n%s", refused.errorOutput)
	}
}

func TestValuesThatBreakTheInputsSchemaAreReportedTogetherAndWriteNothing(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	refused := runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=bad", "--value", "limits.size=x")
	expectRun(testingContext, refused, 1)
	for _, expected := range []string{"`size` must be `integer`", "`mock_id` must match pattern `^m[0-9]+$`", "2 errors"} {
		if !strings.Contains(refused.errorOutput, expected) {
			testingContext.Fatalf("errors lack %q:\n%s", expected, refused.errorOutput)
		}
	}
	if _, err := os.Stat(filepath.Join(repository.Root, "mocks")); err == nil {
		testingContext.Fatal("refused values wrote a target")
	}
}

func TestUpdateAppliesValuesLeavesFilesChangedOutOfBandAloneAndReadsANewerRelease(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	repository.Git("tag", "--annotate", "zerox-templates/v1.0.0", "--message", "mock release")
	runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1", "--ref", "v1.0.0")
	repository.WriteFile("mocks/m1/mock.yml", "edited\n")
	expectRun(testingContext, runIn(testingContext, filepath.Join(repository.Root, "mocks"), "update", "m1", "--value", "limits.size=3"), 0,
		"left alone mocks/m1/mock.yml, changed since zerox wrote it",
		"unchanged mocks/m1/sub/gone.txt",
		"written mocks/m1/zerox.yml")

	repository.Git("rm", "--quiet", "zerox/mock-set/sub/gone.txt")
	repository.WriteFile("zerox/mock-set/added.txt", "added [[ .limits.size ]]\n")
	repository.CommitAll("newer templates")
	repository.Git("tag", "--annotate", "zerox-templates/v1.1.0", "--message", "mock release")
	repository.Git("tag", "--annotate", "zerox-templates/v1", "--message", "mock floating major")
	expectRun(testingContext, runIn(testingContext, repository.Root, "update", "mocks/m1", "--ref", "v1"), 0,
		"written mocks/m1/added.txt",
		"left alone mocks/m1/mock.yml, changed since zerox wrote it",
		"removed mocks/m1/sub/gone.txt, its template is gone",
		"written mocks/m1/zerox.yml")
	if actual := repository.ReadFile("mocks/m1/added.txt"); actual != "added 3\n" {
		testingContext.Fatalf("added.txt: %s", actual)
	}
	if config := repository.ReadFile("mocks/m1/zerox.yml"); !strings.Contains(config, "template: mock-set@v1\n") {
		testingContext.Fatalf("zerox.yml:\n%s", config)
	}
	expectRun(testingContext, runIn(testingContext, repository.Root, "update", "mocks/m1"), 0,
		"unchanged mocks/m1/added.txt",
		"left alone mocks/m1/mock.yml, changed since zerox wrote it",
		"unchanged mocks/m1/zerox.yml")
}

func TestAnUpdateKeepsItsTargetAndNeedsOne(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1")
	moved := runIn(testingContext, repository.Root, "update", "mocks/m1", "--value", "mock_id=m2")
	expectRun(testingContext, moved, 1)
	if !strings.Contains(moved.errorOutput, "the values render the target `mocks/m2`, not `mocks/m1`; an update keeps its target") {
		testingContext.Fatalf("errors:\n%s", moved.errorOutput)
	}
	repository.WriteFile("plain/file.txt", "plain\n")
	plain := runIn(testingContext, repository.Root, "update", "plain")
	expectRun(testingContext, plain, 1)
	if !strings.Contains(plain.errorOutput, "`plain` holds no zerox.yml, so it is not a target; `zerox generate` makes one") {
		testingContext.Fatalf("errors:\n%s", plain.errorOutput)
	}
	outside := runIn(testingContext, repository.Root, "update", "..")
	expectRun(testingContext, outside, 1)
	if !strings.Contains(outside.errorOutput, "`..` is not a folder in the repository") {
		testingContext.Fatalf("errors:\n%s", outside.errorOutput)
	}
}

func TestWhatACommandCannotApplyIsRefusedAndWritesNothing(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1")
	before := repository.ReadFile("mocks/m1/zerox.yml")
	for _, refusal := range []struct {
		workingFolder string
		arguments     []string
		expected      string
	}{
		{repository.Root, []string{"update", "mocks/m1", "--value", "limits.size=x"}, "`size` must be `integer`"},
		{repository.Root, []string{"update", "mocks/m1", "--value", "limits=1", "--value", "limits.size=2"},
			"`limits` holds a value that is not a map, so `limits.size` cannot be set"},
		{repository.Root, []string{"update", "mocks/m1", "--ref", "v9"}, "the repository has no tag `zerox-templates/v9`"},
		{repository.Root, []string{"update", "absent"}, "`absent` is not a folder"},
		{testingContext.TempDir(), []string{"update", "."}, "is not in a git repository's working tree"},
	} {
		refused := runIn(testingContext, refusal.workingFolder, refusal.arguments...)
		expectRun(testingContext, refused, 1)
		if !strings.Contains(refused.errorOutput, refusal.expected) {
			testingContext.Fatalf("%q errors:\n%s", refusal.arguments, refused.errorOutput)
		}
	}
	if repository.ReadFile("mocks/m1/zerox.yml") != before {
		testingContext.Fatal("a refused command rewrote zerox.yml")
	}
	repository.WriteFile("mocks/m1/zerox.yml", "version: \"1\"\ntemplate: mock-set@main\n")
	for _, arguments := range [][]string{
		{"update", "mocks/m1"},
		{"generate", "mock-set", "--value", "mock_id=m1"},
	} {
		refused := runIn(testingContext, repository.Root, arguments...)
		expectRun(testingContext, refused, 1)
		if !strings.Contains(refused.errorOutput, "mocks/m1/zerox.yml") {
			testingContext.Fatalf("%q errors:\n%s", arguments, refused.errorOutput)
		}
	}
}

func TestARefThatIsNotACommitOrVersionIsRefused(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	for arguments, expected := range map[string]string{
		"main":   "`--ref main` is not a ref zerox reads: a commit, `vX.Y.Z` or `vX`",
		"v9":     "the repository has no tag `zerox-templates/v9`",
		"abcdef": "`abcdef` names no commit in the repository",
	} {
		refused := runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "mock_id=m1", "--ref", arguments)
		expectRun(testingContext, refused, 1)
		if !strings.Contains(refused.errorOutput, expected) {
			testingContext.Fatalf("--ref %s errors:\n%s", arguments, refused.errorOutput)
		}
	}
}

func TestTheCommandLineAnswersItsVersionAndRefusesUsageItDoesNotRead(testingContext *testing.T) {
	repository := templatesRepository(testingContext)
	versionRun := runIn(testingContext, repository.Root, "--version")
	expectRun(testingContext, versionRun, 0)
	if !strings.Contains(versionRun.output+versionRun.errorOutput, "mock-version") {
		testingContext.Fatalf("version: %q %q", versionRun.output, versionRun.errorOutput)
	}
	expectRun(testingContext, runIn(testingContext, repository.Root, "update"), 2)
	expectRun(testingContext, runIn(testingContext, testingContext.TempDir(), "generate", "mock-set"), 1)
	expectRun(testingContext, runIn(testingContext, repository.Root, "generate", "mock-set", "--value", "not-an-assignment"), 1)
	expectRun(testingContext, runIn(testingContext, repository.Root, "generate", "missing-set"), 1)
}

func fingerprintOf(content string) string { return targetfiles.Fingerprint([]byte(content)) }
