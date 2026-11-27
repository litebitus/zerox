package zerox

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
	"github.com/litebitus/yaml-dsl-spec/go/diagnostic_renderer"
)

// problemReport renders the problems of one source as a compiler reports them: each message, where it is in the source, and the lines around it.
type problemReport struct {
	sourceName string
	text       string
}

func (report problemReport) of(problems []yamldslspec.Diagnostic) error {
	ordered := slices.Clone(problems)
	slices.SortStableFunc(ordered, compareProblemPositions)
	source := diagnostic_renderer.DiagnosticSource{Name: report.sourceName, Text: report.text}
	rendered := make([]string, 0, len(ordered)+1)
	for _, problem := range ordered {
		problem.Severity = yamldslspec.SeverityError
		rendered = append(rendered, diagnostic_renderer.RenderDiagnostic(problem, source, diagnostic_renderer.RenderStyle{}))
	}
	problemCount := "1 error"
	if len(ordered) != 1 {
		problemCount = fmt.Sprintf("%d errors", len(ordered))
	}
	return errors.New(strings.Join(append(rendered, problemCount), "\n\n"))
}

func (report problemReport) at(position yamldslspec.Position, message string) error {
	return report.of([]yamldslspec.Diagnostic{yamldslspec.DiagnosticAt(position, message)})
}

func compareProblemPositions(first, second yamldslspec.Diagnostic) int {
	switch {
	case first.Position == nil && second.Position == nil:
		return 0
	case first.Position == nil:
		return -1
	case second.Position == nil:
		return 1
	}
	return cmp.Or(cmp.Compare(first.Position.Line, second.Position.Line), cmp.Compare(first.Position.Column, second.Position.Column))
}

// quotedAlternatives are values written as alternatives, each quoted and fenced: `"a"`, `"b"` or `"c"`.
func quotedAlternatives(values []string) string {
	var quoted []string
	for _, value := range values {
		quoted = append(quoted, "`"+strconv.Quote(value)+"`")
	}
	if len(quoted) <= 1 {
		return strings.Join(quoted, "")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}
