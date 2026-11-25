// Package targetfiles writes a target's rendered files, leaving alone each file updated out of band, as its recorded fingerprints show.
package targetfiles

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	zerox "github.com/litebitus/zerox/client/go"
	"github.com/litebitus/zerox/internal/templaterendering"
)

// Outcome is what writing did to one file.
type Outcome int

const (
	Written Outcome = iota
	Unchanged
	Removed
	LeftAloneChanged
	LeftAloneRemoved
	LeftAloneUnwritten
	LeftAloneTemplateGone
)

// Line is the outcome said of a file at a path.
func (outcome Outcome) Line(filePath string) string {
	switch outcome {
	case Written:
		return "written " + filePath
	case Unchanged:
		return "unchanged " + filePath
	case Removed:
		return "removed " + filePath + ", its template is gone"
	case LeftAloneChanged:
		return "left alone " + filePath + ", changed since zerox wrote it"
	case LeftAloneRemoved:
		return "left alone " + filePath + ", removed since zerox wrote it"
	case LeftAloneUnwritten:
		return "left alone " + filePath + ", zerox did not write it"
	}
	return "left alone " + filePath + ", its template is gone and it changed since zerox wrote it"
}

// FileOutcome is a file's path from the target and what writing did to it.
type FileOutcome struct {
	Path    string
	Outcome Outcome
}

// Report is what writing did: each file's outcome, ordered by path, and the fingerprints the target records from then on.
type Report struct {
	Files        []FileOutcome
	Fingerprints map[string]string
}

// Changed tells whether writing wrote or removed a file, or changed a fingerprint from those recorded.
func (report Report) Changed(recorded map[string]string) bool {
	for _, file := range report.Files {
		if file.Outcome == Written || file.Outcome == Removed {
			return true
		}
	}
	return !maps.Equal(report.Fingerprints, recorded)
}

// Write writes the rendered files into the target folder against the fingerprints recorded when zerox last wrote it.
func Write(targetFolder string, rendered []templaterendering.RenderedFile, recorded map[string]string) (Report, error) {
	report := Report{Fingerprints: map[string]string{}}
	renderedPaths := map[string]bool{}
	for _, file := range rendered {
		renderedPaths[file.Path] = true
		outcome, fingerprint, err := writeFile(targetFolder, file, recorded[file.Path])
		if err != nil {
			return Report{}, err
		}
		if fingerprint != "" {
			report.Fingerprints[file.Path] = fingerprint
		}
		report.Files = append(report.Files, FileOutcome{Path: file.Path, Outcome: outcome})
	}
	for _, recordedPath := range slices.Sorted(maps.Keys(recorded)) {
		if renderedPaths[recordedPath] {
			continue
		}
		outcome, isReported, err := removeFile(targetFolder, recordedPath, recorded[recordedPath])
		if err != nil {
			return Report{}, err
		}
		if isReported {
			report.Files = append(report.Files, FileOutcome{Path: recordedPath, Outcome: outcome})
		}
	}
	slices.SortStableFunc(report.Files, func(left, right FileOutcome) int {
		return strings.Compare(left.Path, right.Path)
	})
	return report, nil
}

func writeFile(targetFolder string, file templaterendering.RenderedFile, recordedFingerprint string) (Outcome, string, error) {
	filePath := filepath.Join(targetFolder, filepath.FromSlash(file.Path))
	current, err := os.ReadFile(filePath)
	isPresent := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, "", fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	renderedFingerprint := Fingerprint(file.Content)
	switch {
	case recordedFingerprint != "" && !isPresent:
		return LeftAloneRemoved, recordedFingerprint, nil
	case recordedFingerprint != "" && Fingerprint(current) != recordedFingerprint:
		return LeftAloneChanged, recordedFingerprint, nil
	case isPresent && bytes.Equal(current, file.Content):
		return Unchanged, renderedFingerprint, nil
	case recordedFingerprint == "" && isPresent:
		return LeftAloneUnwritten, "", nil
	}
	if err := writeAtomically(filePath, file.Content, file.Executable); err != nil {
		return 0, "", err
	}
	return Written, renderedFingerprint, nil
}

func removeFile(targetFolder, recordedPath, recordedFingerprint string) (Outcome, bool, error) {
	filePath := filepath.Join(targetFolder, filepath.FromSlash(recordedPath))
	current, err := os.ReadFile(filePath)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	if Fingerprint(current) != recordedFingerprint {
		return LeftAloneTemplateGone, true, nil
	}
	if err := os.Remove(filePath); err != nil {
		return 0, false, fmt.Errorf("failed to remove %s: %w", filePath, err)
	}
	return Removed, true, nil
}

// writeAtomically writes a file through a temporary file beside it, so a reader sees the old content or the new, never part of it.
func writeAtomically(filePath string, content []byte, isExecutable bool) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("failed to make the folder of %s: %w", filePath, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(filePath), ".zerox-*")
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}
	permissions := fs.FileMode(0o644)
	if isExecutable {
		permissions = 0o755
	}
	_, writeErr := temporary.Write(content)
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr, os.Chmod(temporary.Name(), permissions)); err != nil {
		return errors.Join(fmt.Errorf("failed to write %s: %w", filePath, err), os.Remove(temporary.Name()))
	}
	if err := os.Rename(temporary.Name(), filePath); err != nil {
		return errors.Join(fmt.Errorf("failed to write %s: %w", filePath, err), os.Remove(temporary.Name()))
	}
	return nil
}

// WriteConfig writes the target's `zerox.yml`.
func WriteConfig(targetFolder string, target zerox.Target) error {
	text, err := configText(target)
	if err != nil {
		return err
	}
	return writeAtomically(filepath.Join(targetFolder, zerox.ConfigFileName), text, false)
}
