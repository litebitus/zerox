// Package committree reads a git repository's templates folder as one commit holds it, never the working tree.
package committree

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"testing/fstest"
	"time"

	zerox "github.com/litebitus/zerox/client/go"
)

const (
	gitCommandTimeout = time.Minute
	archiveSizeLimit  = 64 << 20
)

// Repository is a git repository's working tree, at its root.
type Repository struct {
	Root string
}

// Open is the repository whose working tree holds a folder.
func Open(folder string) (Repository, error) {
	root, err := gitOutput(folder, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, fmt.Errorf("%s is not in a git repository's working tree: %w", folder, err)
	}
	return Repository{Root: strings.TrimSpace(string(root))}, nil
}

// Commit is the full commit a ref names, as git resolves it.
func (repository Repository) Commit(ref string) (string, error) {
	commit, err := gitOutput(repository.Root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("`%s` names no commit in the repository", ref)
	}
	return strings.TrimSpace(string(commit)), nil
}

// ShortCommit is a commit abbreviated as `git rev-parse --short` abbreviates it.
func (repository Repository) ShortCommit(commit string) (string, error) {
	short, err := gitOutput(repository.Root, "rev-parse", "--short", commit)
	if err != nil {
		return "", fmt.Errorf("failed to abbreviate commit %s: %w", commit, err)
	}
	return strings.TrimSpace(string(short)), nil
}

// TemplatesAt are the templates folder's files as a commit holds them, at their paths from the repository's root.
func (repository Repository) TemplatesAt(commit string) (fs.FS, error) {
	archive, err := gitOutput(repository.Root, "archive", "--format=tar", commit, "--", zerox.TemplatesFolder)
	if err != nil {
		return nil, fmt.Errorf("commit %s holds no `%s` folder: %w", commit, zerox.TemplatesFolder, err)
	}
	files := fstest.MapFS{}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read the templates of commit %s: %w", commit, err)
		}
		switch header.Typeflag {
		case tar.TypeDir, tar.TypeXGlobalHeader:
			continue
		case tar.TypeReg:
			content, err := io.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s of commit %s: %w", header.Name, commit, err)
			}
			files[header.Name] = &fstest.MapFile{Data: content, Mode: fs.FileMode(header.Mode).Perm()}
		default:
			return nil, fmt.Errorf("%s of commit %s is not a regular file, so it is not a template", header.Name, commit)
		}
	}
}

func gitOutput(folder string, arguments ...string) ([]byte, error) {
	commandContext, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, "git", append([]string{"-C", folder}, arguments...)...)
	var output, errorOutput bytes.Buffer
	command.Stdout = &limitedBuffer{buffer: &output, limit: archiveSizeLimit}
	command.Stderr = &limitedBuffer{buffer: &errorOutput, limit: 1 << 16}
	if err := command.Run(); err != nil {
		if message := strings.TrimSpace(errorOutput.String()); message != "" {
			return nil, errors.New(message)
		}
		return nil, err
	}
	return output.Bytes(), nil
}

// limitedBuffer holds a command's output up to a limit, and fails the command past it.
type limitedBuffer struct {
	buffer *bytes.Buffer
	limit  int
}

func (limited *limitedBuffer) Write(written []byte) (int, error) {
	if limited.buffer.Len()+len(written) > limited.limit {
		return 0, fmt.Errorf("the output is larger than %d bytes", limited.limit)
	}
	return limited.buffer.Write(written)
}
