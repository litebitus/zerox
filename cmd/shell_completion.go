package cmd

import (
	_ "embed"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// completeAfterCursorVariable is the environment variable the zsh function hands the words after the cursor in,
// one per line.
const completeAfterCursorVariable = "ZEROX_COMPLETE_AFTER_CURSOR"

//go:embed completion.zsh
var zshCompletionFunction string

var zshCompletionTemplate = template.Must(template.New("zsh").Parse(zshCompletionFunction))

// completionScripts writes each shell's script: zsh zerox's own function, the others cobra's generated ones.
var completionScripts = map[string]func(root *cobra.Command, output io.Writer, described bool) error{
	"bash": func(root *cobra.Command, output io.Writer, described bool) error {
		return root.GenBashCompletionV2(output, described)
	},
	"fish": func(root *cobra.Command, output io.Writer, described bool) error {
		return root.GenFishCompletion(output, described)
	},
	"powershell": func(root *cobra.Command, output io.Writer, described bool) error {
		if described {
			return root.GenPowerShellCompletionWithDesc(output)
		}
		return root.GenPowerShellCompletion(output)
	},
	"zsh": func(_ *cobra.Command, output io.Writer, described bool) error {
		completionRequest := cobra.ShellCompNoDescRequestCmd
		if described {
			completionRequest = cobra.ShellCompRequestCmd
		}
		return zshCompletionTemplate.Execute(output, struct{ Request string }{completionRequest})
	},
}

// ownCompletionScripts mounts cobra's completion command and has each shell write its script where the
// command's output goes.
func ownCompletionScripts(root *cobra.Command) {
	root.InitDefaultCompletionCmd()
	for _, command := range root.Commands() {
		if command.Name() != "completion" {
			continue
		}
		for _, shellCommand := range command.Commands() {
			writeScript, known := completionScripts[shellCommand.Name()]
			if !known {
				continue
			}
			shellCommand.RunE = func(invokedCommand *cobra.Command, _ []string) error {
				noDescriptions, _ := invokedCommand.Flags().GetBool("no-descriptions")
				return failureOfCommand(writeScript(invokedCommand.Root(), invokedCommand.OutOrStdout(), !noDescriptions))
			}
		}
	}
}

// completeTargetFolder completes `update`'s target from the folders.
func completeTargetFolder(_ *cobra.Command, arguments []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(arguments) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// completionRequestArguments is the arguments the root runs with: a completion request's with the flags after
// the cursor, and their values, set ahead of the word being completed, so a flag already on the line is known
// to the completion; any other arguments as they are.
func completionRequestArguments(root *cobra.Command, arguments []string) []string {
	afterCursor := os.Getenv(completeAfterCursorVariable)
	if len(arguments) < 2 || afterCursor == "" ||
		(arguments[0] != cobra.ShellCompRequestCmd && arguments[0] != cobra.ShellCompNoDescRequestCmd) {
		return arguments
	}
	beforeWord, completedWord := arguments[1:len(arguments)-1], arguments[len(arguments)-1]
	command, _, err := root.Find(beforeWord)
	if err != nil {
		return arguments
	}
	flags := flagsAfterCursor(command, strings.Split(afterCursor, "\n"))
	insertAt := len(beforeWord)
	if insertAt > 0 && awaitsValue(command, beforeWord[insertAt-1]) {
		insertAt--
	}
	requestArguments := append([]string{arguments[0]}, beforeWord[:insertAt]...)
	requestArguments = append(requestArguments, flags...)
	requestArguments = append(requestArguments, beforeWord[insertAt:]...)
	return append(requestArguments, completedWord)
}

// flagsAfterCursor is the command's flags among the words after the cursor, each with the value it takes;
// arguments and words the command defines no flag for are left out.
func flagsAfterCursor(command *cobra.Command, words []string) []string {
	var flags []string
	for wordIndex := 0; wordIndex < len(words); wordIndex++ {
		word := words[wordIndex]
		if word == "--" {
			break
		}
		if flagNamedBy(command, word) == nil {
			continue
		}
		flags = append(flags, word)
		if awaitsValue(command, word) && wordIndex+1 < len(words) {
			flags = append(flags, words[wordIndex+1])
			wordIndex++
		}
	}
	return flags
}

// awaitsValue is whether a word is a flag of the command whose value is the next word.
func awaitsValue(command *cobra.Command, word string) bool {
	flag := flagNamedBy(command, word)
	return flag != nil && flag.NoOptDefVal == "" && !strings.Contains(word, "=")
}

// flagNamedBy is the command's flag a word names, by its long or short form, with its value attached or not;
// nil for a word that names none.
func flagNamedBy(command *cobra.Command, word string) *pflag.Flag {
	name, _, _ := strings.Cut(word, "=")
	for _, flags := range []*pflag.FlagSet{command.Flags(), command.InheritedFlags()} {
		switch {
		case strings.HasPrefix(name, "--") && len(name) > 2:
			if flag := flags.Lookup(name[2:]); flag != nil {
				return flag
			}
		case strings.HasPrefix(name, "-") && len(name) == 2 && name != "--":
			if flag := flags.ShorthandLookup(name[1:]); flag != nil {
				return flag
			}
		}
	}
	return nil
}
