// Package valueassignments reads the command line's `--value <path>=<value>` and sets each value at its input's path.
package valueassignments

import (
	"fmt"
	"regexp"
	"strings"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

var inputNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParseValueAssignment reads `<path>=<value>`: an input's path, its names joined by `.`, and its value as YAML.
func ParseValueAssignment(assignment string) (string, any, error) {
	inputPath, valueText, isAssigned := strings.Cut(assignment, "=")
	if !isAssigned {
		return "", nil, fmt.Errorf("`%s` is not `<path>=<value>`", assignment)
	}
	if err := checkInputPath(inputPath); err != nil {
		return "", nil, err
	}
	root, readProblem := yamldslspec.ParseYAML(inputPath, valueText, nil)
	if readProblem != nil {
		return "", nil, fmt.Errorf("the value of `%s` is not YAML: %s", inputPath, readProblem.Message)
	}
	return inputPath, root.ToJSON(), nil
}

func checkInputPath(inputPath string) error {
	for _, name := range strings.Split(inputPath, ".") {
		if !inputNamePattern.MatchString(name) {
			return fmt.Errorf("`%s` is not an input's path: each name is a letter or `_`, then letters, digits and `_`, joined by `.`", inputPath)
		}
	}
	return nil
}

// SetValue sets the value at an input's path, making each map on the way to it.
func SetValue(values map[string]any, inputPath string, value any) error {
	if err := checkInputPath(inputPath); err != nil {
		return err
	}
	names := strings.Split(inputPath, ".")
	holder := values
	for index, name := range names[:len(names)-1] {
		next, isGiven := holder[name]
		if !isGiven || next == nil {
			created := map[string]any{}
			holder[name] = created
			holder = created
			continue
		}
		nested, isMap := next.(map[string]any)
		if !isMap {
			return fmt.Errorf("`%s` holds a value that is not a map, so `%s` cannot be set", strings.Join(names[:index+1], "."), inputPath)
		}
		holder = nested
	}
	holder[names[len(names)-1]] = value
	return nil
}
