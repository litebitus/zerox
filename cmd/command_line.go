// Package cmd is zerox's command line: its commands, their flags, and what each reports.
package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/alecthomas/kingpin/v2"
)

// exitRequest is kingpin asking to end the program, as after `--help` or `--version`.
type exitRequest struct{ code int }

// Run runs the command line's arguments from the working folder, and answers the exit code.
func Run(version string, arguments []string, workingFolder string, output, errorOutput io.Writer,
	now func() time.Time) (exitCode int) {
	application := kingpin.New("zerox", "Renders a template set into a target folder, and renders a target again from its zerox.yml.")
	application.Version(version)
	application.UsageWriter(output)
	application.ErrorWriter(errorOutput)
	application.Terminate(func(code int) { panic(exitRequest{code: code}) })
	defer func() {
		if recovered := recover(); recovered != nil {
			request, isExit := recovered.(exitRequest)
			if !isExit {
				panic(recovered)
			}
			exitCode = request.code
		}
	}()

	generate := application.Command("generate", "Render a template set into a new target, from the values given and the defaults.")
	generateSet := generate.Arg("set", "The template set, a folder under zerox/.").Required().String()
	generateValues := generate.Flag("value", "A value, <path>=<value>, the value read as YAML; repeatable.").PlaceHolder("PATH=VALUE").Strings()
	generateRef := generate.Flag("ref", "The ref the templates are read at: a commit, vX.Y.Z or vX; HEAD without it.").String()

	update := application.Command("update", "Render a target again from its zerox.yml, with the values given over its values.")
	updateTarget := update.Arg("target", "The target folder.").Required().String()
	updateValues := update.Flag("value", "A value, <path>=<value>, the value read as YAML; repeatable.").PlaceHolder("PATH=VALUE").Strings()
	updateRef := update.Flag("ref", "The ref the templates are read at: a commit, vX.Y.Z or vX; the lineage's ref without it.").String()

	command, err := application.Parse(arguments)
	if err != nil {
		application.Errorf("%s", err)
		return 2
	}
	request := renderRequest{workingFolder: workingFolder, output: output, now: now}
	switch command {
	case generate.FullCommand():
		request.ref, request.assignments = *generateRef, *generateValues
		err = request.generate(*generateSet)
	case update.FullCommand():
		request.ref, request.assignments = *updateRef, *updateValues
		err = request.update(*updateTarget)
	}
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return 1
	}
	return 0
}
