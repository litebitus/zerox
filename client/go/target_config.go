package zerox

import (
	"fmt"
	"io/fs"
	"regexp"
	"time"
)

// RefPattern is a ref templates are read at: an abbreviated or full commit, a release as `v1.2.3`, or a floating major as `v1`.
var RefPattern = regexp.MustCompile(`^([0-9a-f]{4,40}|v[0-9]+(\.[0-9]+\.[0-9]+)?)$`)

// Lineage is a template set and the ref its templates were read at, written `<set>@<ref>`.
type Lineage struct {
	Set string
	Ref string
}

// ParseLineage reads a lineage written `<set>@<ref>`.
func ParseLineage(text string) (Lineage, error) {
	for index := len(text) - 1; index >= 0; index-- {
		if text[index] != '@' {
			continue
		}
		lineage := Lineage{Set: text[:index], Ref: text[index+1:]}
		if SetNamePattern.MatchString(lineage.Set) && RefPattern.MatchString(lineage.Ref) {
			return lineage, nil
		}
		break
	}
	return Lineage{}, fmt.Errorf("`%s` is not a lineage, `<set>@<ref>`, its ref a commit, `vX.Y.Z` or `vX`", text)
}

func (lineage Lineage) String() string { return lineage.Set + "@" + lineage.Ref }

// Target is a target's `zerox.yml`: the lineage it was rendered from, when it last changed, every value applied, each input's tags, and each file's fingerprint.
type Target struct {
	Lineage      Lineage
	GeneratedAt  time.Time
	Values       map[string]any
	InputTags    map[string][]string
	Fingerprints map[string]string
}

// ReadTarget reads a target's `zerox.yml` at a path of files.
func ReadTarget(files fs.FS, filePath string) (Target, error) {
	file, err := readConfigFile(files, filePath, targetConfig)
	if err != nil {
		return Target{}, err
	}
	templateNode, _ := file.root.Get("template")
	lineage, err := ParseLineage(templateNode.Text)
	if err != nil {
		return Target{}, file.report.at(templateNode.Position, err.Error())
	}
	generatedAtNode, _ := file.root.Get("generated_at")
	generatedAt, err := time.Parse(time.RFC3339, generatedAtNode.Text)
	if err != nil {
		return Target{}, file.report.at(generatedAtNode.Position, fmt.Sprintf("`generated_at` `%s` is not a time", generatedAtNode.Text))
	}
	valuesNode, _ := file.root.Get("values")
	inputTagsNode, _ := file.root.Get("input_tags")
	fingerprintsNode, _ := file.root.Get("fingerprints")
	target := Target{
		Lineage:      lineage,
		GeneratedAt:  generatedAt,
		Values:       valuesNode.ToJSON().(map[string]any),
		InputTags:    map[string][]string{},
		Fingerprints: map[string]string{},
	}
	for _, entry := range inputTagsNode.Entries {
		target.InputTags[entry.Key] = textsOf(entry.Value)
	}
	for _, entry := range fingerprintsNode.Entries {
		target.Fingerprints[entry.Key] = entry.Value.Text
	}
	return target, nil
}
