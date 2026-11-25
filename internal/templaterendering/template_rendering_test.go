package templaterendering

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	zerox "github.com/litebitus/zerox/client/go"
)

const mockSetConfig = `version: "1"
delimiters: {left: "[[", right: "]]"}
default_tags: [operator]
target: mocks/[[ .mock_id ]]
inputs:
  type: object
  properties:
    mock_id: {type: string}
    limits:
      type: object
      properties:
        size: {type: integer, default: 10}
        names: {type: array, items: {type: string}}
`

func mockTemplates(extraFiles map[string]string) fstest.MapFS {
	files := fstest.MapFS{"zerox/mock-set/zerox.yml": {Data: []byte(mockSetConfig)}}
	for filePath, content := range extraFiles {
		files[filePath] = &fstest.MapFile{Data: []byte(content)}
	}
	return files
}

func readMockSet(testingContext *testing.T, files fstest.MapFS) zerox.TemplateSet {
	testingContext.Helper()
	set, err := zerox.ReadTemplateSet(files, "mock-set")
	if err != nil {
		testingContext.Fatal(err)
	}
	return set
}

func expectError(testingContext *testing.T, err error, expectedParts ...string) {
	testingContext.Helper()
	if err == nil {
		testingContext.Fatalf("no error, expected one holding %q", expectedParts)
	}
	for _, expected := range expectedParts {
		if !strings.Contains(err.Error(), expected) {
			testingContext.Fatalf("error lacks %q:\n%s", expected, err)
		}
	}
}

func TestEveryTemplateRendersToItsPathUnderTheTargetAndTheConfigIsNotATemplate(testingContext *testing.T) {
	files := mockTemplates(map[string]string{
		"zerox/mock-set/mock.yml":      "id: [[ .mock_id ]]\nkeep: {{MOCK_PLACEHOLDER}}\n",
		"zerox/mock-set/sub/zerox.yml": "names:[[- range .limits.names ]] [[ . ]][[- end ]]\n",
		"zerox/mock-set/sub/when.txt":  "[[ if eq .limits.size 10 ]]ten[[ else ]]other[[ end ]]\n",
		"zerox/other-set/ignored.txt":  "ignored",
	})
	files["zerox/mock-set/run.sh"] = &fstest.MapFile{Data: []byte("echo [[ .mock_id ]]\n"), Mode: 0o755}
	set := readMockSet(testingContext, files)
	values := set.CompletedValues(map[string]any{"mock_id": "m1", "limits": map[string]any{"names": []any{"a", "b"}}})
	rendered, err := RenderFiles(files, set, values)
	if err != nil {
		testingContext.Fatal(err)
	}
	expected := []RenderedFile{
		{Path: "mock.yml", Content: []byte("id: m1\nkeep: {{MOCK_PLACEHOLDER}}\n")},
		{Path: "run.sh", Content: []byte("echo m1\n"), Executable: true},
		{Path: "sub/when.txt", Content: []byte("ten\n")},
		{Path: "sub/zerox.yml", Content: []byte("names: a b\n")},
	}
	if !reflect.DeepEqual(rendered, expected) {
		testingContext.Fatalf("rendered %+v", rendered)
	}
}

func TestEveryTemplateThatFailsIsReportedTogether(testingContext *testing.T) {
	files := mockTemplates(map[string]string{
		"zerox/mock-set/missing.txt": "[[ .absent ]]",
		"zerox/mock-set/broken.txt":  "[[ if ]]",
	})
	set := readMockSet(testingContext, files)
	_, err := RenderFiles(files, set, map[string]any{"mock_id": "m1"})
	expectError(testingContext, err, "zerox/mock-set/missing.txt", `map has no entry for key "absent"`, "zerox/mock-set/broken.txt")
}

func TestASetMissingFromTheTemplatesGivenIsReported(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	_, err := RenderFiles(fstest.MapFS{}, set, map[string]any{"mock_id": "m1"})
	expectError(testingContext, err, "failed to read the templates of `mock-set`")
}

func TestTheTargetRendersToAFolderOutsideTheTemplates(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	if targetPath, err := RenderTargetPath(set, map[string]any{"mock_id": "m1"}); err != nil || targetPath != "mocks/m1" {
		testingContext.Fatalf("target %q: %v", targetPath, err)
	}
	for _, target := range []string{"", ".", "..", "../out", "/abs", "a//b", "zerox", "zerox/inside", "a/../b"} {
		set.Target = target
		_, err := RenderTargetPath(set, map[string]any{})
		expectError(testingContext, err, "`target` renders to `"+target+"`")
	}
	set.Target = "[[ .absent ]]"
	_, err := RenderTargetPath(set, map[string]any{})
	expectError(testingContext, err, "absent")
}
