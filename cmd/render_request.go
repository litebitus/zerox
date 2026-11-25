package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	zerox "github.com/litebitus/zerox/client/go"
	"github.com/litebitus/zerox/internal/committree"
	"github.com/litebitus/zerox/internal/targetfiles"
	"github.com/litebitus/zerox/internal/templaterendering"
	"github.com/litebitus/zerox/internal/valueassignments"
)

// renderRequest is one command's request: where it runs, the ref and values it was given, and where it reports.
type renderRequest struct {
	workingFolder string
	ref           string
	assignments   []string
	output        io.Writer
	now           func() time.Time
}

func (request renderRequest) generate(setName string) error {
	repository, err := committree.Open(request.workingFolder)
	if err != nil {
		return err
	}
	lineageRef, commit, err := request.resolvedRef(repository, "")
	if err != nil {
		return err
	}
	templates, set, err := readSet(repository, commit, setName)
	if err != nil {
		return err
	}
	values, err := request.checkedValues(set, map[string]any{})
	if err != nil {
		return err
	}
	targetPath, err := templaterendering.RenderTargetPath(set, values)
	if err != nil {
		return err
	}
	previous, err := readPrevious(repository, targetPath)
	if err != nil {
		return err
	}
	if previous != nil && !reflect.DeepEqual(previous.Values, values) {
		return fmt.Errorf("`%s` was generated with other values; `zerox update %s` changes them", targetPath, targetPath)
	}
	return request.renderInto(repository, templates, set,
		zerox.Lineage{Set: setName, Ref: lineageRef}, targetPath, values, previous)
}

func (request renderRequest) update(targetFolder string) error {
	repository, err := committree.Open(request.workingFolder)
	if err != nil {
		return err
	}
	targetPath, err := repositoryPath(repository, request.workingFolder, targetFolder)
	if err != nil {
		return err
	}
	previous, err := readPrevious(repository, targetPath)
	if err != nil {
		return err
	}
	if previous == nil {
		return fmt.Errorf("`%s` holds no %s, so it is not a target; `zerox generate` makes one", targetPath, zerox.ConfigFileName)
	}
	lineageRef, commit, err := request.resolvedRef(repository, previous.Lineage.Ref)
	if err != nil {
		return err
	}
	templates, set, err := readSet(repository, commit, previous.Lineage.Set)
	if err != nil {
		return err
	}
	values, err := request.checkedValues(set, previous.Values)
	if err != nil {
		return err
	}
	renderedPath, err := templaterendering.RenderTargetPath(set, values)
	if err != nil {
		return err
	}
	if renderedPath != targetPath {
		return fmt.Errorf("the values render the target `%s`, not `%s`; an update keeps its target", renderedPath, targetPath)
	}
	return request.renderInto(repository, templates, set,
		zerox.Lineage{Set: previous.Lineage.Set, Ref: lineageRef}, targetPath, values, previous)
}

// resolvedRef is the ref a lineage records and the commit it names: the ref given, else the lineage's, else `HEAD` recorded as its commit.
func (request renderRequest) resolvedRef(repository committree.Repository, lineageRef string) (string, string, error) {
	ref := request.ref
	if ref != "" && !zerox.RefPattern.MatchString(ref) {
		return "", "", fmt.Errorf("`--ref %s` is not a ref zerox reads: a commit, `vX.Y.Z` or `vX`", ref)
	}
	if ref == "" {
		ref = lineageRef
	}
	if ref != "" {
		commit, err := repository.Commit(ref)
		return ref, commit, err
	}
	commit, err := repository.Commit("HEAD")
	if err != nil {
		return "", "", err
	}
	shortCommit, err := repository.ShortCommit(commit)
	return shortCommit, commit, err
}

// readSet is the templates a commit holds and the template set read from them.
func readSet(repository committree.Repository, commit, setName string) (fs.FS, zerox.TemplateSet, error) {
	templates, err := repository.TemplatesAt(commit)
	if err != nil {
		return nil, zerox.TemplateSet{}, err
	}
	set, err := zerox.ReadTemplateSet(templates, setName)
	return templates, set, err
}

func (request renderRequest) checkedValues(set zerox.TemplateSet, recorded map[string]any) (map[string]any, error) {
	values := set.CompletedValues(recorded)
	for _, assignment := range request.assignments {
		inputPath, value, err := valueassignments.ParseValueAssignment(assignment)
		if err != nil {
			return nil, err
		}
		if err := valueassignments.SetValue(values, inputPath, value); err != nil {
			return nil, err
		}
	}
	values = set.CompletedValues(values)
	return values, set.CheckValues(values)
}

func readPrevious(repository committree.Repository, targetPath string) (*zerox.Target, error) {
	configPath := path.Join(targetPath, zerox.ConfigFileName)
	if _, err := os.Stat(filepath.Join(repository.Root, filepath.FromSlash(configPath))); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	target, err := zerox.ReadTarget(os.DirFS(repository.Root), configPath)
	if err != nil {
		return nil, err
	}
	return &target, nil
}

// repositoryPath is a folder named from the working folder, as a path from the repository's root.
func repositoryPath(repository committree.Repository, workingFolder, folder string) (string, error) {
	absolute := folder
	if !filepath.IsAbs(folder) {
		absolute = filepath.Join(workingFolder, folder)
	}
	root, err := filepath.EvalSymlinks(repository.Root)
	if err != nil {
		return "", fmt.Errorf("failed to read the repository's root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("`%s` is not a folder: %w", folder, err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("`%s` is not a folder in the repository", folder)
	}
	return filepath.ToSlash(relative), nil
}

func (request renderRequest) renderInto(repository committree.Repository, templates fs.FS, set zerox.TemplateSet,
	lineage zerox.Lineage, targetPath string, values map[string]any, previous *zerox.Target) error {
	rendered, err := templaterendering.RenderFiles(templates, set, values)
	if err != nil {
		return err
	}
	recorded := map[string]string{}
	if previous != nil {
		recorded = previous.Fingerprints
	}
	targetFolder := filepath.Join(repository.Root, filepath.FromSlash(targetPath))
	report, err := targetfiles.Write(targetFolder, rendered, recorded)
	if err != nil {
		return err
	}
	for _, file := range report.Files {
		fmt.Fprintln(request.output, file.Outcome.Line(path.Join(targetPath, file.Path)))
	}
	target := zerox.Target{Lineage: lineage, Values: values, InputTags: set.InputTags(), Fingerprints: report.Fingerprints}
	configPath := path.Join(targetPath, zerox.ConfigFileName)
	if previous != nil && !report.Changed(recorded) && sameRecord(*previous, target) {
		fmt.Fprintln(request.output, targetfiles.Unchanged.Line(configPath))
		return nil
	}
	target.GeneratedAt = request.now().UTC().Truncate(time.Second)
	if err := targetfiles.WriteConfig(targetFolder, target); err != nil {
		return err
	}
	fmt.Fprintln(request.output, targetfiles.Written.Line(configPath))
	return nil
}

func sameRecord(previous, current zerox.Target) bool {
	return previous.Lineage == current.Lineage &&
		reflect.DeepEqual(previous.Values, current.Values) &&
		maps.EqualFunc(previous.InputTags, current.InputTags, slices.Equal[[]string]) &&
		maps.Equal(previous.Fingerprints, current.Fingerprints)
}
