// Package cmd is zerox's command line: its commands, their flags, and what each reports.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// commandFailure is the error of a command that ran and failed, as against a command line zerox does not read.
type commandFailure struct{ cause error }

func (failure commandFailure) Error() string { return failure.cause.Error() }

// failureOfCommand marks a command's error as its failure; nil stays nil.
func failureOfCommand(err error) error {
	if err == nil {
		return nil
	}
	return commandFailure{cause: err}
}

// Run runs the command line's arguments from the working folder, reading a confirmation from input where it is a
// terminal, and answers the exit code.
func Run(version string, arguments []string, workingFolder string, input io.Reader, inputIsTerminal bool,
	output, errorOutput io.Writer, now func() time.Time) int {
	request := renderRequest{
		workingFolder:   workingFolder,
		input:           input,
		inputIsTerminal: inputIsTerminal,
		output:          output,
		errorOutput:     errorOutput,
		now:             now,
	}
	root := commandTree(version, &request)
	root.SetOut(output)
	root.SetErr(errorOutput)
	root.SetArgs(completionRequestArguments(root, arguments))
	err := root.Execute()
	if err == nil {
		return 0
	}
	fmt.Fprintln(errorOutput, err)
	if errors.As(err, &commandFailure{}) {
		return 1
	}
	return 2
}

// commandTree is zerox's root command with its commands, each filling request from its argument and flags.
func commandTree(version string, request *renderRequest) *cobra.Command {
	root := &cobra.Command{
		Use:           "zerox",
		Short:         "Renders a template set into a target folder, and renders a target again from its zerox.yml.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().BoolVar(&request.isForced, "force", false,
		"Do what a command refuses to do by default, after a warning of it and a confirmation.")
	root.PersistentFlags().BoolVar(&request.isConfirmed, "yes", false,
		"Answer yes to every prompt a command asks, as a workflow needs.")
	root.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)

	generate := &cobra.Command{
		Use:   "generate <set>",
		Short: "Render a template set into a new target, from the values given and the defaults.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, arguments []string) error {
			return failureOfCommand(request.generate(arguments[0]))
		},
	}
	generate.Flags().StringArrayVar(&request.assignments, "value", nil,
		"A value, <path>=<value>, the value read as YAML; repeatable.")
	generate.Flags().StringVar(&request.ref, "ref", "",
		"The ref the templates are read at; HEAD without it.")

	update := &cobra.Command{
		Use:               "update <target>",
		Short:             "Render a target again from its zerox.yml, with the values given over its values.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTargetFolder,
		RunE: func(_ *cobra.Command, arguments []string) error {
			return failureOfCommand(request.update(arguments[0]))
		},
	}
	update.Flags().StringArrayVar(&request.assignments, "value", nil,
		"A value, <path>=<value>, the value read as YAML; repeatable.")
	update.Flags().StringVar(&request.ref, "ref", "",
		"The ref the templates are read at; the lineage's ref without it.")

	root.AddCommand(generate, update)
	ownCompletionScripts(root)
	return root
}
