package zerox

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

// SetNamePattern is a template set's name: a folder name under the templates folder.
var SetNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Delimiters are the text a template action starts and ends with.
type Delimiters struct {
	Left  string
	Right string
}

// TemplateSet is a template set's `zerox.yml` read with the common config merged under it.
type TemplateSet struct {
	Name        string
	Delimiters  Delimiters
	DefaultTags []string
	Target      string
	inputs      yamldslspec.Node
	inputSchema *yamldslspec.LoadedSchema
}

// ReadTemplateSet reads the template set `setName` from files holding a repository's root: its `zerox.yml`, and the common config merged under it where the repository has one.
func ReadTemplateSet(files fs.FS, setName string) (TemplateSet, error) {
	if !SetNamePattern.MatchString(setName) {
		return TemplateSet{}, fmt.Errorf("`%s` is not a template set's name, a folder name under `%s/`", setName, TemplatesFolder)
	}
	setPath := path.Join(TemplatesFolder, setName, ConfigFileName)
	if _, err := fs.Stat(files, setPath); err != nil {
		return TemplateSet{}, fmt.Errorf("`%s` is not a template set: %s is not in the templates read", setName, setPath)
	}
	setFile, err := readConfigFile(files, setPath, templatesConfig)
	if err != nil {
		return TemplateSet{}, err
	}
	merged := setFile.root
	commonPath := path.Join(TemplatesFolder, ConfigFileName)
	if _, err := fs.Stat(files, commonPath); err == nil {
		commonFile, err := readConfigFile(files, commonPath, templatesConfig)
		if err != nil {
			return TemplateSet{}, err
		}
		merged = mergedNode(commonFile.root, setFile.root)
	}
	return templateSetOf(setName, setFile, merged)
}

// mergedNode is the common config with a set's config over it: maps merge key by key, and the set's value wins anywhere else.
func mergedNode(common, set yamldslspec.Node) yamldslspec.Node {
	if common.Kind != yamldslspec.NodeMap || set.Kind != yamldslspec.NodeMap {
		return set
	}
	merged := set
	merged.Entries = nil
	for _, commonEntry := range common.Entries {
		if setValue, isSet := set.Get(commonEntry.Key); isSet {
			commonEntry.Value = mergedNode(commonEntry.Value, setValue)
		}
		merged.Entries = append(merged.Entries, commonEntry)
	}
	for _, setEntry := range set.Entries {
		if _, isCommon := common.Get(setEntry.Key); !isCommon {
			merged.Entries = append(merged.Entries, setEntry)
		}
	}
	return merged
}

func templateSetOf(setName string, setFile configFile, merged yamldslspec.Node) (TemplateSet, error) {
	for _, field := range []string{"delimiters", "default_tags", "target", "inputs"} {
		if _, isStated := merged.Get(field); !isStated {
			return TemplateSet{}, setFile.report.at(setFile.root.Position,
				fmt.Sprintf("`%s` is stated by neither this file nor `%s/%s`", field, TemplatesFolder, ConfigFileName))
		}
	}
	delimiters, _ := merged.Get("delimiters")
	left, _ := delimiters.Get("left")
	right, _ := delimiters.Get("right")
	defaultTags, _ := merged.Get("default_tags")
	target, _ := merged.Get("target")
	inputs, _ := merged.Get("inputs")
	inputsJSON, err := json.Marshal(inputs.ToJSON())
	if err != nil {
		return TemplateSet{}, fmt.Errorf("failed to read the inputs of `%s`: %w", setName, err)
	}
	inputSchema, err := yamldslspec.NewSchemaCache().Load(path.Join(TemplatesFolder, setName, ConfigFileName, "inputs"), string(inputsJSON))
	if err != nil {
		return TemplateSet{}, fmt.Errorf("the inputs of `%s` are not a JSON Schema: %w", setName, err)
	}
	return TemplateSet{
		Name:        setName,
		Delimiters:  Delimiters{Left: left.Text, Right: right.Text},
		DefaultTags: textsOf(defaultTags),
		Target:      target.Text,
		inputs:      inputs,
		inputSchema: inputSchema,
	}, nil
}

func textsOf(listNode yamldslspec.Node) []string {
	texts := make([]string, 0, len(listNode.Items))
	for _, item := range listNode.Items {
		texts = append(texts, item.Text)
	}
	return texts
}
