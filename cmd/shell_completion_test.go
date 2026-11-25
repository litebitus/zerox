package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// zshStubs stand in for zsh's completion system, printing what the function hands it; mock_zerox answers
// __complete with MOCK_REPLY.
const zshStubs = `
compdef() { : }
_describe() {
    local order tag
    [[ $1 == -V ]] && { order=$1; shift }
    [[ $1 == -t ]] && { tag=$2; shift 2 }
    shift
    local name=$1
    shift
    print -r -- "describe $tag order=$order options=${(j: :)${(@qq)@}}"
    print -rl -- "${(@P)name}"
}
compadd() {
    local -a said
    while (( $# )); do
        if [[ $1 == -[ad] ]]; then
            said+=("$1" "${(j:, :)${(@P)2}}")
            shift 2
        else
            said+=("$1")
            shift
        fi
    done
    print -r -- "compadd ${(j: :)${(@qq)said}}"
}
compset() { print -r -- "compset ${(j: :)${(@qq)@}}" }
_files() { print -r -- "files ${(j: :)${(@qq)@}}" }
mock_zerox() {
    print -r -- "args ${(j: :)${(@qq)@}}" >> "$MOCK_ARGS"
    print -r -- "$MOCK_REPLY"
}
`

// matcherOption is what the function hands zsh to match a run typed anywhere in a candidate, and in each part
// of a token built from several.
const matcherOption = "'-M' 'l:|=* l:[:/]|=* r:|=*'"

// zshCompletionOf runs the zsh function for a line whose current word is prefix, the binary answering reply,
// and answers what it handed zsh and what the binary was asked.
func zshCompletionOf(testingContext *testing.T, words []string, prefix, reply string) (
	handedToZsh, askedOfBinary string) {
	testingContext.Helper()
	quotedWords := make([]string, len(words))
	for wordIndex, word := range words {
		quotedWords[wordIndex] = "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
	}
	scratchFolder := testingContext.TempDir()
	argumentsFile := filepath.Join(scratchFolder, "args")
	handedToZsh = sourcedInZsh(testingContext, scratchFolder,
		"words=(mock_zerox "+strings.Join(quotedWords, " ")+")\n"+
			"CURRENT=${#words}\n"+
			"PREFIX='"+strings.ReplaceAll(prefix, "'", `'\''`)+"'\n"+
			"_zerox\nprint -r -- \"status $?\"\n",
		"MOCK_REPLY="+reply, "MOCK_ARGS="+argumentsFile)
	asked, _ := os.ReadFile(argumentsFile)
	return handedToZsh, strings.TrimSpace(string(asked))
}

// sourcedInZsh runs lines in zsh after the stubs and the function `zerox completion zsh` prints, and answers
// what they printed.
func sourcedInZsh(testingContext *testing.T, scratchFolder, lines string, environment ...string) string {
	testingContext.Helper()
	if _, err := exec.LookPath("zsh"); err != nil {
		testingContext.Fatal("zsh is required: these tests run the completion function in it")
	}
	function := runIn(testingContext, scratchFolder, "completion", "zsh")
	if function.exitCode != 0 {
		testingContext.Fatalf("exit %d, error output %q", function.exitCode, function.errorOutput)
	}
	script := filepath.Join(scratchFolder, "_zerox")
	if err := os.WriteFile(script, []byte(function.output), 0o644); err != nil {
		testingContext.Fatal(err)
	}
	zsh := exec.Command("zsh", "-f", "-c", zshStubs+"source "+script+"\n"+lines)
	zsh.Env = append(os.Environ(), environment...)
	printed, err := zsh.CombinedOutput()
	if err != nil {
		testingContext.Fatalf("zsh: %v\n%s", err, printed)
	}
	return string(printed)
}

func TestTheZshStylesAreZeroxsOwn(testingContext *testing.T) {
	said := sourcedInZsh(testingContext, testingContext.TempDir(), "zstyle -L\n")
	expected := "zstyle ':completion:*:*:*zerox:*:values' list-grouped false\n" +
		"zstyle ':completion:*:*:*zerox:*' menu select\n"
	if said != expected {
		testingContext.Fatalf("styles:\n%s\nexpected:\n%s", said, expected)
	}
}

func TestZshListsAFlagsFormsOnOneLineAndValuesApart(testingContext *testing.T) {
	said, _ := zshCompletionOf(testingContext, []string{"generate", "-"}, "-",
		"--help\thelp for mock\n-h\thelp for mock\n--ref\tref for mock\n-x\tone form\nmock-value\tmock\n:4")
	expected := "compadd '-J' 'options' '-l' '-d' " +
		"'--help | -h  -- help for mock, --ref        -- ref for mock, -x           -- one form' " +
		"'-Q' " + matcherOption + " '-a' '--help, --ref, -x'\n" +
		"compadd '-J' 'options' '-n' '-Q' " + matcherOption + " '-a' '-h'\n" +
		"describe values order= options='-Q' " + matcherOption + "\n" +
		"mock-value:mock\n" +
		"status 0\n"
	if said != expected {
		testingContext.Errorf("handed zsh:\n%s\nexpected:\n%s", said, expected)
	}
}

func TestZshTypesNoSharedRun(testingContext *testing.T) {
	for _, state := range []struct{ prefix, unambiguous, insert, oldList, matches, expected string }{
		{"", "2", "automenu-unambiguous", "", "4", ""},
		{"", "2", "automenu-unambiguous", "yes", "4", "menu"},
		{"2", "24", "unambiguous", "", "2", ""},
		{"", "23", "automenu-unambiguous", "", "1", "automenu-unambiguous"},
	} {
		said := sourcedInZsh(testingContext, testingContext.TempDir(), fmt.Sprintf(
			"typeset -A compstate\ncompstate=(unambiguous %q insert %q old_list %q nmatches %q)\nPREFIX=%q\n"+
				"__zerox_type_no_shared_run\nprint -r -- \"[${compstate[insert]}]\"\n",
			state.unambiguous, state.insert, state.oldList, state.matches, state.prefix))
		if strings.TrimSpace(said) != "["+state.expected+"]" {
			testingContext.Errorf("%+v: insert is %s", state, said)
		}
	}
	scratchFolder := testingContext.TempDir()
	said := sourcedInZsh(testingContext, scratchFolder,
		"words=(mock_zerox generate)\nCURRENT=2\nPREFIX=''\n_zerox >/dev/null\n"+
			"print -r -- \"$comppostfuncs\"\n",
		"MOCK_REPLY=mock-candidate\n:4", "MOCK_ARGS="+filepath.Join(scratchFolder, "args"))
	if strings.TrimSpace(said) != "__zerox_type_no_shared_run" {
		testingContext.Errorf("the function does not hand zsh its check: %q", said)
	}
}

func TestZshListsALoneDescription(testingContext *testing.T) {
	for _, state := range []struct{ matches, expected string }{
		{"1", "list force"},
		{"2", "autolist"},
	} {
		said := sourcedInZsh(testingContext, testingContext.TempDir(), fmt.Sprintf(
			"typeset -A compstate\ncompstate=(nmatches %q list autolist)\n"+
				"__zerox_list_a_lone_description\nprint -r -- \"[${compstate[list]}]\"\n", state.matches))
		if strings.TrimSpace(said) != "["+state.expected+"]" {
			testingContext.Errorf("%s matches: list is %s, expected %q", state.matches, said, state.expected)
		}
	}
	for reply, expected := range map[string]string{
		"mock-candidate\tmock description\n:4":       "__zerox_type_no_shared_run __zerox_list_a_lone_description",
		"mock-candidate\n:4":                         "__zerox_type_no_shared_run",
		"mock-one\tmock one\nmock-two\tmock two\n:4": "__zerox_type_no_shared_run",
	} {
		scratchFolder := testingContext.TempDir()
		said := sourcedInZsh(testingContext, scratchFolder,
			"words=(mock_zerox generate)\nCURRENT=2\nPREFIX=''\n_zerox >/dev/null\n"+
				"print -r -- \"$comppostfuncs\"\n",
			"MOCK_REPLY="+reply, "MOCK_ARGS="+filepath.Join(scratchFolder, "args"))
		if strings.TrimSpace(said) != expected {
			testingContext.Errorf("replied %q: handed zsh %q, expected %q", reply, strings.TrimSpace(said), expected)
		}
	}
}

func TestZshListsOnlyWhatComesNext(testingContext *testing.T) {
	said, asked := zshCompletionOf(testingContext, []string{"generate", "mock-set", "--value", "mocks/"}, "mocks/",
		"mocks/m1\tmock one\nmocks/m2/\tmock two\n:6")
	expected := "compset '-P' 'mocks/'\n" +
		"describe values order= options='-Q' " + matcherOption + " '-S' ''\n" +
		"m1:mock one\n" +
		"m2/:mock two\n" +
		"status 0\n"
	if said != expected {
		testingContext.Errorf("handed zsh:\n%s\nexpected:\n%s", said, expected)
	}
	if asked != "args '__complete' 'generate' 'mock-set' '--value' 'mocks/'" {
		testingContext.Errorf("asked %s", asked)
	}
}

func TestZshAsksForAWordInAnOpenQuoteWithoutTheQuote(testingContext *testing.T) {
	_, asked := zshCompletionOf(testingContext,
		[]string{"generate", "mock-set", "--value", "'mock_id=m"}, "mock_id=m", "mock_id=m1\tmock\n:6")
	if asked != "args '__complete' 'generate' 'mock-set' '--value' 'mock_id=m'" {
		testingContext.Errorf("asked %s", asked)
	}
}

func TestZshSetsTheTypedPartAsideAtEachSeparator(testingContext *testing.T) {
	said, _ := zshCompletionOf(testingContext, []string{"generate", "mock-set", "--value", "mocks/m1?"}, "mocks/m1?",
		"mocks/m1?mock=\tmock query\n:6")
	if !strings.Contains(said, "compset '-P' 'mocks/m1\\?'\n") || !strings.Contains(said, "\nmock=:mock query\n") {
		testingContext.Errorf("after a ?:\n%s", said)
	}
	said, _ = zshCompletionOf(testingContext, []string{"generate", "mock-set", "--value=mocks/"}, "--value=mocks/",
		"mocks/m1\nmocks/m2\n:4")
	if !strings.Contains(said, "compset '-P' '--value=mocks/'\n") || !strings.Contains(said, "\nm1\nm2\n") {
		testingContext.Errorf("a flag with its value:\n%s", said)
	}
}

func TestZshListsCandidatesWholeInTheOrderGiven(testingContext *testing.T) {
	said, _ := zshCompletionOf(testingContext, []string{"generate", "mock-set", "--value", "mo"}, "mo",
		"mock:m1\tmock value\n:36")
	expected := "describe values order=-V options='-Q' " + matcherOption + "\n" +
		"mock\\:m1:mock value\n" +
		"status 0\n"
	if said != expected {
		testingContext.Errorf("handed zsh:\n%s\nexpected:\n%s", said, expected)
	}
}

func TestZshFollowsTheDirectives(testingContext *testing.T) {
	helped, _ := zshCompletionOf(testingContext, []string{"generate"}, "", "_activeHelp_ mock help at 100%\n:4")
	if helped != "compadd '-x' '%F{11}mock help at 100%%%f'\nstatus 1\n" {
		testingContext.Errorf("active help:\n%s", helped)
	}
	for reply, expected := range map[string]string{
		":1":          "status 1\n",
		":0":          "files \nstatus 0\n",
		":16":         "files '-/'\nstatus 0\n",
		"not a reply": "status 1\n",
	} {
		if said, _ := zshCompletionOf(testingContext, []string{"update"}, "", reply); said != expected {
			testingContext.Errorf("replied %q:\n%s", reply, said)
		}
	}
}

func TestTheCompletionCommandPrintsEachShellsScript(testingContext *testing.T) {
	for shell, expected := range map[string]string{
		"bash":       "__start_zerox",
		"fish":       "complete -c zerox",
		"powershell": "Register-ArgumentCompleter -CommandName 'zerox'",
		"zsh":        "#compdef zerox\n",
	} {
		for _, noDescriptions := range []bool{false, true} {
			arguments := []string{"completion", shell}
			completionRequest := cobra.ShellCompRequestCmd + " "
			if noDescriptions {
				arguments = append(arguments, "--no-descriptions")
				completionRequest = cobra.ShellCompNoDescRequestCmd
			}
			run := runIn(testingContext, testingContext.TempDir(), arguments...)
			if run.exitCode != 0 || run.errorOutput != "" {
				testingContext.Fatalf("%q: exit %d, error output %q", arguments, run.exitCode, run.errorOutput)
			}
			if !strings.Contains(run.output, expected) || !strings.Contains(run.output, completionRequest) {
				testingContext.Errorf("%q printed:\n%s", arguments, run.output)
			}
		}
	}
	zshFunction := runIn(testingContext, testingContext.TempDir(), "completion", "zsh")
	if !strings.HasPrefix(zshFunction.output, "#compdef zerox\n") ||
		!strings.Contains(zshFunction.output, "${~${(Q)words[1]}} __complete ") {
		testingContext.Errorf("completion zsh printed:\n%s", zshFunction.output)
	}
}

func TestZshHandsTheWordsAfterTheCursor(testingContext *testing.T) {
	scratchFolder := testingContext.TempDir()
	argumentsFile := filepath.Join(scratchFolder, "args")
	sourcedInZsh(testingContext, scratchFolder, `mock_zerox() {
    print -r -- "args ${(j: :)${(@qq)@}}" >> "$MOCK_ARGS"
    print -r -- "after ${ZEROX_COMPLETE_AFTER_CURSOR//$'\n'/|}" >> "$MOCK_ARGS"
    print -r -- ":4"
}
words=(mock_zerox generate mock-set --ref '' --value 'mock_id=m1' -h)
CURRENT=5
PREFIX=''
_zerox
print -r -- "status $?"
`, "MOCK_ARGS="+argumentsFile)
	asked, _ := os.ReadFile(argumentsFile)
	expected := "args '__complete' 'generate' 'mock-set' '--ref' ''\nafter --value|mock_id=m1|-h\n"
	if string(asked) != expected {
		testingContext.Fatalf("asked:\n%s\nexpected:\n%s", asked, expected)
	}
}

// mockCompletionTree is a root with one command whose --env completes to the envs of the stack on the line and
// whose arguments complete to their position.
func mockCompletionTree() *cobra.Command {
	root := &cobra.Command{Use: "mock-zerox"}
	deploy := &cobra.Command{
		Use: "mock-deploy",
		Run: func(*cobra.Command, []string) {},
		ValidArgsFunction: func(_ *cobra.Command, arguments []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{fmt.Sprintf("mock-position-%d", len(arguments))}, cobra.ShellCompDirectiveNoFileComp
		},
	}
	deploy.Flags().StringP("env", "e", "", "mock env")
	deploy.Flags().String("stack", "", "mock stack")
	deploy.Flags().BoolP("yes", "y", false, "mock yes")
	_ = deploy.RegisterFlagCompletionFunc("env",
		func(command *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			stack, _ := command.Flags().GetString("stack")
			if stack == "" {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{stack + "-dev"}, cobra.ShellCompDirectiveNoFileComp
		})
	root.AddCommand(deploy)
	return root
}

// mockTreeAnswer is what the mock tree answers a completion request with the words after the cursor given.
func mockTreeAnswer(testingContext *testing.T, afterCursor string, request ...string) string {
	testingContext.Helper()
	testingContext.Setenv(completeAfterCursorVariable, afterCursor)
	root := mockCompletionTree()
	var answer strings.Builder
	root.SetOut(&answer)
	root.SetArgs(completionRequestArguments(root, append([]string{cobra.ShellCompRequestCmd}, request...)))
	if err := root.Execute(); err != nil {
		testingContext.Fatal(err)
	}
	return answer.String()
}

func TestFlagsAfterTheCursorAreNotOfferedAgain(testingContext *testing.T) {
	answer := mockTreeAnswer(testingContext, "--stack\nmock-stack\n-y", "mock-deploy", "-e", "mock-env", "-")
	for _, flag := range []string{"--stack", "--yes", "--env"} {
		if strings.Contains(answer, flag) {
			testingContext.Errorf("%s is on the line and offered again:\n%s", flag, answer)
		}
	}
}

func TestAValueCompletesFromAFlagAfterTheCursor(testingContext *testing.T) {
	answer := mockTreeAnswer(testingContext, "--stack\nmock-stack", "mock-deploy", "-e", "")
	if !strings.Contains(answer, "mock-stack-dev\n") {
		testingContext.Errorf("--env completes from the stack after the cursor:\n%s", answer)
	}
}

func TestAnArgumentAfterTheCursorShiftsNoPosition(testingContext *testing.T) {
	answer := mockTreeAnswer(testingContext, "mock-later-arg\n--stack\nmock-stack", "mock-deploy", "")
	if !strings.Contains(answer, "mock-position-0\n") {
		testingContext.Errorf("the first argument is completed as the first:\n%s", answer)
	}
}

func TestCompletionRequestArgumentsOtherwiseStandAsTheyAre(testingContext *testing.T) {
	root := mockCompletionTree()
	testingContext.Setenv(completeAfterCursorVariable, "")
	request := []string{cobra.ShellCompRequestCmd, "mock-deploy", "-e", ""}
	if actual := completionRequestArguments(root, request); strings.Join(actual, " ") != strings.Join(request, " ") {
		testingContext.Errorf("arguments = %q", actual)
	}
	testingContext.Setenv(completeAfterCursorVariable, "--stack\nmock-stack")
	run := []string{"mock-deploy", "-e", "mock-env"}
	if actual := completionRequestArguments(root, run); strings.Join(actual, " ") != strings.Join(run, " ") {
		testingContext.Errorf("arguments = %q", actual)
	}
}

func TestACommandsFlagsAreCompletedWithTheirDescriptions(testingContext *testing.T) {
	for command, refDescription := range map[string]string{
		"generate": "The ref the templates are read at: a commit, vX.Y.Z or vX; HEAD without it.",
		"update":   "The ref the templates are read at: a commit, vX.Y.Z or vX; the lineage's ref without it.",
	} {
		run := runIn(testingContext, testingContext.TempDir(), cobra.ShellCompRequestCmd, command, "-")
		for _, expectedLine := range []string{
			"--ref\t" + refDescription + "\n",
			"--value\tA value, <path>=<value>, the value read as YAML; repeatable.\n",
		} {
			if !strings.Contains(run.output, expectedLine) {
				testingContext.Errorf("%s - completes no %q:\n%s", command, expectedLine, run.output)
			}
		}
	}
	testingContext.Setenv(completeAfterCursorVariable, "--ref\nv1")
	run := runIn(testingContext, testingContext.TempDir(), cobra.ShellCompRequestCmd, "update", "-")
	if strings.Contains(run.output, "--ref") {
		testingContext.Errorf("--ref is on the line and offered again:\n%s", run.output)
	}
}

func TestUpdateCompletesItsTargetFromTheFoldersAndGenerateCompletesNothing(testingContext *testing.T) {
	for arguments, expected := range map[string]string{
		"update|":          ":16\n",
		"update|mocks/m1|": ":4\n",
		"generate|":        ":4\n",
	} {
		request := append([]string{cobra.ShellCompRequestCmd}, strings.Split(arguments, "|")...)
		run := runIn(testingContext, testingContext.TempDir(), request...)
		if run.output != expected {
			testingContext.Errorf("%q answered %q, expected %q", arguments, run.output, expected)
		}
	}
}
