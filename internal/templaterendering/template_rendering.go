// Package templaterendering renders a template set's target path and templates with the values, by Go's `text/template` and the set's delimiters.
package templaterendering

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"text/template"

	zerox "github.com/litebitus/zerox/client/go"
)

// RenderedFile is a template rendered: its path from the target, its content, and whether it is executable.
type RenderedFile struct {
	Path       string
	Content    []byte
	Executable bool
}

// RenderTargetPath is the target folder's path from the repository's root, the set's `target` rendered with the values.
func RenderTargetPath(set zerox.TemplateSet, values map[string]any) (string, error) {
	rendered, err := render(set.Delimiters, "target", set.Target, values)
	if err != nil {
		return "", err
	}
	targetPath := string(rendered)
	cleaned := path.Clean(targetPath)
	if targetPath == "" || cleaned != targetPath || path.IsAbs(targetPath) || targetPath == "." ||
		targetPath == ".." || strings.HasPrefix(targetPath, "../") ||
		targetPath == zerox.TemplatesFolder || strings.HasPrefix(targetPath, zerox.TemplatesFolder+"/") {
		return "", fmt.Errorf("`target` renders to `%s`, which is not a folder's path from the repository's root outside `%s/`",
			targetPath, zerox.TemplatesFolder)
	}
	return targetPath, nil
}

// RenderFiles renders every template of the set in the templates with the values, ordered by path; a template that fails is reported with every other that fails.
func RenderFiles(templates fs.FS, set zerox.TemplateSet, values map[string]any) ([]RenderedFile, error) {
	setFolder := path.Join(zerox.TemplatesFolder, set.Name)
	var rendered []RenderedFile
	var failures []error
	err := fs.WalkDir(templates, setFolder, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativePath := strings.TrimPrefix(filePath, setFolder+"/")
		if entry.IsDir() || relativePath == zerox.ConfigFileName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		text, err := fs.ReadFile(templates, filePath)
		if err != nil {
			return err
		}
		content, err := render(set.Delimiters, filePath, string(text), values)
		if err != nil {
			failures = append(failures, err)
			return nil
		}
		rendered = append(rendered, RenderedFile{Path: relativePath, Content: content, Executable: info.Mode()&0o111 != 0})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read the templates of `%s`: %w", set.Name, err)
	}
	if len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	slices.SortFunc(rendered, func(left, right RenderedFile) int { return strings.Compare(left.Path, right.Path) })
	return rendered, nil
}

func render(delimiters zerox.Delimiters, templateName, text string, values map[string]any) ([]byte, error) {
	parsed, err := template.New(templateName).Delims(delimiters.Left, delimiters.Right).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, values); err != nil {
		return nil, err
	}
	return rendered.Bytes(), nil
}
