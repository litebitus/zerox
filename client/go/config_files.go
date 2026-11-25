// Package zerox reads zerox's config files and renders a template set: the templates' `zerox.yml` and the common config merged under it, a target's `zerox.yml`, the inputs with their tags, and the values checked against the inputs' schema.
package zerox

import (
	"fmt"
	"io"
	"io/fs"
	"strconv"

	yamldslspec "github.com/litebitus/yaml-dsl-spec/go"
)

const (
	// ConfigFileName is the name of every zerox config file.
	ConfigFileName = "zerox.yml"
	// TemplatesFolder is the folder at a repository's root holding the template sets and the common config.
	TemplatesFolder = "zerox"
	// ConfigFileSizeLimit is the largest config file zerox reads.
	ConfigFileSizeLimit = 1 << 20
)

var configSchemas = yamldslspec.NewSchemaCache()

// configFile is one zerox.yml as read and held to its version's schema.
type configFile struct {
	report problemReport
	root   yamldslspec.Node
}

// configKind tells a target's zerox.yml, which states its lineage, from a template set's or the common config's.
type configKind int

const (
	templatesConfig configKind = iota
	targetConfig
)

func readConfigFile(files fs.FS, filePath string, kind configKind) (configFile, error) {
	opened, err := files.Open(filePath)
	if err != nil {
		return configFile{}, fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	defer opened.Close()
	text, err := io.ReadAll(io.LimitReader(opened, ConfigFileSizeLimit+1))
	if err != nil {
		return configFile{}, fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	if len(text) > ConfigFileSizeLimit {
		return configFile{}, fmt.Errorf("%s is larger than 1 MiB", filePath)
	}
	return parseConfigFile(filePath, string(text), kind)
}

func parseConfigFile(filePath, text string, kind configKind) (configFile, error) {
	report := problemReport{sourceName: filePath, text: text}
	root, readProblem := yamldslspec.ParseYAML(filePath, text, nil)
	if readProblem != nil {
		return configFile{}, report.of([]yamldslspec.Diagnostic{*readProblem})
	}
	if root.Kind != yamldslspec.NodeMap {
		return configFile{}, report.at(root.Position, "a `zerox.yml` is a mapping of its fields")
	}
	templateNode, isTargetConfig := root.Get("template")
	if kind == templatesConfig && isTargetConfig {
		return configFile{}, report.at(templateNode.Position, "a template set's `zerox.yml` states no `template`; a target's does")
	}
	if kind == targetConfig && !isTargetConfig {
		return configFile{}, report.at(root.Position, "a target's `zerox.yml` states `template`, its lineage")
	}
	versionNode, isWritten := root.Get("version")
	stated, isText := versionNode.AsString()
	if !isText {
		versionPosition := root.Position
		if isWritten {
			versionPosition = versionNode.Position
		}
		return configFile{}, report.at(versionPosition, "`version` is required, quoted, one of "+quotedAlternatives(StatedVersions()))
	}
	schemaText, minorVersion, err := SchemaFor(stated)
	if err != nil {
		return configFile{}, report.at(versionNode.Position, fmt.Sprintf("`version` `%s` is not a version zerox reads; it reads %s",
			strconv.Quote(stated), quotedAlternatives(StatedVersions())))
	}
	loadedSchema, err := configSchemas.Load("zerox-v"+minorVersion+".schema.json", string(schemaText))
	if err != nil {
		return configFile{}, fmt.Errorf("the schema of version %s cannot be read: %w", minorVersion, err)
	}
	if problems := loadedSchema.Validate(yamldslspec.ResolvedValueFromNode(root), yamldslspec.ValidationRules{}); len(problems) > 0 {
		return configFile{}, report.of(problems)
	}
	return configFile{report: report, root: root}, nil
}
