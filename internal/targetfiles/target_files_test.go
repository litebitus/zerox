package targetfiles

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	zerox "github.com/litebitus/zerox/client/go"
	"github.com/litebitus/zerox/internal/templaterendering"
)

func writeMockFile(testingContext *testing.T, folder, filePath, content string) {
	testingContext.Helper()
	absolute := filepath.Join(folder, filePath)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		testingContext.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		testingContext.Fatal(err)
	}
}

func fingerprintOf(content string) string { return Fingerprint([]byte(content)) }

func TestEachFileIsWrittenOrLeftAloneAsItsFingerprintShows(testingContext *testing.T) {
	folder := testingContext.TempDir()
	writeMockFile(testingContext, folder, "same.txt", "same\n")
	writeMockFile(testingContext, folder, "stale.txt", "old\n")
	writeMockFile(testingContext, folder, "edited.txt", "edited by hand\n")
	writeMockFile(testingContext, folder, "foreign.txt", "not zerox's\n")
	writeMockFile(testingContext, folder, "adopted.txt", "adopted\n")
	writeMockFile(testingContext, folder, "gone.txt", "gone\n")
	writeMockFile(testingContext, folder, "gone-edited.txt", "edited by hand\n")
	recorded := map[string]string{
		"same.txt":         fingerprintOf("same\n"),
		"stale.txt":        fingerprintOf("old\n"),
		"edited.txt":       fingerprintOf("as zerox wrote it\n"),
		"deleted.txt":      fingerprintOf("deleted\n"),
		"gone.txt":         fingerprintOf("gone\n"),
		"gone-edited.txt":  fingerprintOf("as zerox wrote it\n"),
		"gone-missing.txt": fingerprintOf("missing\n"),
	}
	rendered := []templaterendering.RenderedFile{
		{Path: "same.txt", Content: []byte("same\n")},
		{Path: "stale.txt", Content: []byte("new\n")},
		{Path: "edited.txt", Content: []byte("newer\n")},
		{Path: "deleted.txt", Content: []byte("deleted\n")},
		{Path: "foreign.txt", Content: []byte("zerox's\n")},
		{Path: "adopted.txt", Content: []byte("adopted\n")},
		{Path: "new/run.sh", Content: []byte("echo new\n"), Executable: true},
	}
	report, err := Write(folder, rendered, recorded)
	if err != nil {
		testingContext.Fatal(err)
	}
	var lines []string
	for _, file := range report.Files {
		lines = append(lines, file.Outcome.Line(file.Path))
	}
	expectedLines := []string{
		"unchanged adopted.txt",
		"left alone deleted.txt, removed since zerox wrote it",
		"left alone edited.txt, changed since zerox wrote it",
		"left alone foreign.txt, zerox did not write it",
		"left alone gone-edited.txt, its template is gone and it changed since zerox wrote it",
		"removed gone.txt, its template is gone",
		"written new/run.sh",
		"unchanged same.txt",
		"written stale.txt",
	}
	if !reflect.DeepEqual(lines, expectedLines) {
		testingContext.Fatalf("lines:\n%q\nexpected:\n%q", lines, expectedLines)
	}
	expectedFingerprints := map[string]string{
		"adopted.txt": fingerprintOf("adopted\n"),
		"deleted.txt": recorded["deleted.txt"],
		"edited.txt":  recorded["edited.txt"],
		"new/run.sh":  fingerprintOf("echo new\n"),
		"same.txt":    fingerprintOf("same\n"),
		"stale.txt":   fingerprintOf("new\n"),
	}
	if !reflect.DeepEqual(report.Fingerprints, expectedFingerprints) {
		testingContext.Fatalf("fingerprints %v", report.Fingerprints)
	}
	if !report.Changed(recorded) {
		testingContext.Fatal("writing changed nothing")
	}
	if info, err := os.Stat(filepath.Join(folder, "new", "run.sh")); err != nil || info.Mode().Perm() != 0o755 {
		testingContext.Fatalf("run.sh %v: %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "gone.txt")); !os.IsNotExist(err) {
		testingContext.Fatalf("gone.txt was kept: %v", err)
	}
	for filePath, expected := range map[string]string{"edited.txt": "edited by hand\n", "foreign.txt": "not zerox's\n", "stale.txt": "new\n"} {
		if content, _ := os.ReadFile(filepath.Join(folder, filePath)); string(content) != expected {
			testingContext.Fatalf("%s holds %q", filePath, content)
		}
	}
}

func TestWritingWhatIsAlreadyThereChangesNothing(testingContext *testing.T) {
	folder := testingContext.TempDir()
	writeMockFile(testingContext, folder, "same.txt", "same\n")
	recorded := map[string]string{"same.txt": fingerprintOf("same\n")}
	report, err := Write(folder, []templaterendering.RenderedFile{{Path: "same.txt", Content: []byte("same\n")}}, recorded)
	if err != nil || report.Changed(recorded) {
		testingContext.Fatalf("report %+v: %v", report, err)
	}
}

func TestATargetsConfigReadsBackAsZeroxWroteIt(testingContext *testing.T) {
	written := zerox.Target{
		Lineage:      zerox.Lineage{Set: "mock-set", Ref: "v1"},
		GeneratedAt:  time.Date(2026, 11, 25, 8, 0, 0, 0, time.FixedZone("mock", 3600)),
		Values:       map[string]any{"mock_id": "m1", "version": "1.0", "size": int64(10), "on": true, "names": []any{"yes", "null"}},
		InputTags:    map[string][]string{"mock_id": {"requester"}},
		Fingerprints: map[string]string{"mock.yml": fingerprintOf("mock")},
	}
	folder := testingContext.TempDir()
	if err := WriteConfig(folder, written); err != nil {
		testingContext.Fatal(err)
	}
	read, err := zerox.ReadTarget(os.DirFS(folder), zerox.ConfigFileName)
	if err != nil {
		testingContext.Fatal(err)
	}
	if !read.GeneratedAt.Equal(written.GeneratedAt) || read.GeneratedAt.Location() != time.UTC {
		testingContext.Fatalf("generated_at %v", read.GeneratedAt)
	}
	read.GeneratedAt = written.GeneratedAt
	if !reflect.DeepEqual(read, written) {
		testingContext.Fatalf("read %+v\nwritten %+v", read, written)
	}
	if err := WriteConfig(folder, zerox.Target{Lineage: zerox.Lineage{Set: "mock-set", Ref: "abcd"}}); err != nil {
		testingContext.Fatal(err)
	}
	if _, err := zerox.ReadTarget(os.DirFS(folder), zerox.ConfigFileName); err != nil {
		testingContext.Fatal(err)
	}
}

func TestAFileThatCannotBeReadOrWrittenFailsTheWrite(testingContext *testing.T) {
	folder := testingContext.TempDir()
	if err := os.MkdirAll(filepath.Join(folder, "folder.txt"), 0o755); err != nil {
		testingContext.Fatal(err)
	}
	writeMockFile(testingContext, folder, "blocker", "a file where a folder goes\n")
	for _, rendered := range []templaterendering.RenderedFile{
		{Path: "folder.txt", Content: []byte("x")},
		{Path: "blocker/inside.txt", Content: []byte("x")},
	} {
		if _, err := Write(folder, []templaterendering.RenderedFile{rendered}, map[string]string{}); err == nil {
			testingContext.Fatalf("%s was written", rendered.Path)
		}
	}
	if _, err := Write(folder, nil, map[string]string{"folder.txt": fingerprintOf("x")}); err == nil {
		testingContext.Fatal("a folder was read as a file")
	}
}
