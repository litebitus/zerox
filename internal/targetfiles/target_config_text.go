package targetfiles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	zerox "github.com/litebitus/zerox/client/go"
	"go.yaml.in/yaml/v4"
)

type targetFile struct {
	Version      string              `yaml:"version"`
	Template     string              `yaml:"template"`
	GeneratedAt  quotedText          `yaml:"generated_at"`
	Values       map[string]any      `yaml:"values"`
	InputTags    map[string][]string `yaml:"input_tags"`
	Fingerprints map[string]string   `yaml:"fingerprints"`
}

type quotedText string

func (text quotedText) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.DoubleQuotedStyle, Value: string(text)}, nil
}

func configText(target zerox.Target) ([]byte, error) {
	written := targetFile{
		Version:      "1",
		Template:     target.Lineage.String(),
		GeneratedAt:  quotedText(target.GeneratedAt.UTC().Format(time.RFC3339)),
		Values:       nonNil(target.Values),
		InputTags:    nonNil(target.InputTags),
		Fingerprints: nonNil(target.Fingerprints),
	}
	var text bytes.Buffer
	encoder := yaml.NewEncoder(&text)
	encoder.SetIndent(2)
	if err := encoder.Encode(written); err != nil {
		return nil, fmt.Errorf("failed to write the target's %s: %w", zerox.ConfigFileName, err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to write the target's %s: %w", zerox.ConfigFileName, err)
	}
	return text.Bytes(), nil
}

func nonNil[Value any](entries map[string]Value) map[string]Value {
	if entries == nil {
		return map[string]Value{}
	}
	return entries
}

// Fingerprint is the SHA-256 of a file's content, in hex.
func Fingerprint(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
