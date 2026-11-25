package zerox

import (
	"bytes"
	"fmt"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
	"go.yaml.in/yaml/v4"
)

// Input is one input of a template set: its path, its schema, its default where the schema states one, and its resolved tags.
type Input struct {
	Path       string
	Schema     map[string]any
	Default    any
	HasDefault bool
	Tags       []string
}

// Inputs are the template set's inputs in the order the schema writes them, each input followed by the inputs inside it.
func (set TemplateSet) Inputs() []Input {
	var inputs []Input
	set.collectInputs(set.inputs, "", &inputs)
	return inputs
}

func joinedPath(parentPath, name string) string {
	if parentPath == "" {
		return name
	}
	return parentPath + "." + name
}

func (set TemplateSet) collectInputs(schemaNode yamldslspec.Node, parentPath string, inputs *[]Input) {
	properties, _ := schemaNode.Get("properties")
	for _, property := range properties.Entries {
		inputPath := joinedPath(parentPath, property.Key)
		schema, _ := property.Value.ToJSON().(map[string]any)
		input := Input{Path: inputPath, Schema: schema, Tags: set.DefaultTags}
		if defaultNode, isStated := property.Value.Get("default"); isStated {
			input.Default, input.HasDefault = defaultNode.ToJSON(), true
		}
		if tagsNode, isStated := property.Value.Get("tags"); isStated {
			input.Tags = textsOf(tagsNode)
		}
		*inputs = append(*inputs, input)
		set.collectInputs(property.Value, inputPath, inputs)
	}
}

// InputTags are each input's resolved tags, keyed by its path.
func (set TemplateSet) InputTags() map[string][]string {
	tags := map[string][]string{}
	for _, input := range set.Inputs() {
		tags[input.Path] = input.Tags
	}
	return tags
}

// CompletedValues are the values with each input left out given its default, and each object input left out given its properties' defaults, at any depth.
func (set TemplateSet) CompletedValues(values map[string]any) map[string]any {
	completed, _ := copiedValue(values).(map[string]any)
	if completed == nil {
		completed = map[string]any{}
	}
	return completedObject(set.inputs, completed)
}

func completedObject(schemaNode yamldslspec.Node, values map[string]any) map[string]any {
	properties, _ := schemaNode.Get("properties")
	for _, property := range properties.Entries {
		current, isGiven := values[property.Key]
		if isGiven {
			if nested, isObject := current.(map[string]any); isObject {
				values[property.Key] = completedObject(property.Value, nested)
			}
			continue
		}
		if defaultNode, isStated := property.Value.Get("default"); isStated {
			values[property.Key] = defaultNode.ToJSON()
			continue
		}
		if nested := completedObject(property.Value, map[string]any{}); len(nested) > 0 {
			values[property.Key] = nested
		}
	}
	return values
}

// CheckValues holds values to the inputs' schema, every problem reported at once against the values as YAML;
// values that break the schema are answered as InputProblems.
func (set TemplateSet) CheckValues(values map[string]any) error {
	text, err := valuesText(values)
	if err != nil {
		return err
	}
	const sourceName = "the values"
	root, readProblem := yamldslspec.ParseYAML(sourceName, text, nil)
	if readProblem != nil {
		return fmt.Errorf("failed to read the values back: %s", readProblem.Message)
	}
	if problems := set.inputSchema.Problems(yamldslspec.ResolvedValueFromNode(root), yamldslspec.ValidationRules{}); len(problems) > 0 {
		return set.inputProblemsOf(problemReport{sourceName: sourceName, text: text}, problems)
	}
	return nil
}

func valuesText(values map[string]any) (string, error) {
	var text bytes.Buffer
	encoder := yaml.NewEncoder(&text)
	encoder.SetIndent(2)
	if err := encoder.Encode(values); err != nil {
		return "", fmt.Errorf("failed to write the values as YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to write the values as YAML: %w", err)
	}
	return text.String(), nil
}

func copiedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = copiedValue(item)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for index, item := range typed {
			copied[index] = copiedValue(item)
		}
		return copied
	}
	return value
}
