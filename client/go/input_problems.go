package zerox

import (
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

// InputProblem is a problem of one input's value: the input's path and the problem's message.
type InputProblem struct {
	Path    string
	Message string
}

// InputProblems is the error CheckValues answers when the values break the inputs' schema: every input
// problem, and the report of them all its Error answers.
type InputProblems struct {
	Problems []InputProblem
	report   string
}

func (problems InputProblems) Error() string { return problems.report }

// inputProblemsOf are the values' schema problems, each at the path of the input its subject is, or is
// inside, and their report.
func (set TemplateSet) inputProblemsOf(report problemReport, schemaProblems []yamldslspec.SchemaProblem) InputProblems {
	inputPaths := map[string]bool{}
	for _, input := range set.Inputs() {
		inputPaths[input.Path] = true
	}
	diagnostics := make([]yamldslspec.Diagnostic, 0, len(schemaProblems))
	problems := make([]InputProblem, 0, len(schemaProblems))
	for _, schemaProblem := range schemaProblems {
		diagnostics = append(diagnostics, schemaProblem.Diagnostic)
		problems = append(problems, InputProblem{
			Path:    inputPathOf(schemaProblem.Subject, inputPaths),
			Message: schemaProblem.Diagnostic.Message,
		})
	}
	return InputProblems{Problems: problems, report: report.of(diagnostics).Error()}
}

// inputPathOf is the path of the deepest input a JSON pointer names or is inside; a pointer inside no input
// is the values' top, the empty path.
func inputPathOf(pointer string, inputPaths map[string]bool) string {
	if pointer == "" {
		return ""
	}
	names := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for index, name := range names {
		names[index] = strings.ReplaceAll(strings.ReplaceAll(name, "~1", "/"), "~0", "~")
	}
	for count := len(names); count > 0; count-- {
		if candidate := strings.Join(names[:count], "."); inputPaths[candidate] {
			return candidate
		}
	}
	return ""
}
