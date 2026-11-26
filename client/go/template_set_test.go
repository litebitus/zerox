package zerox

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

const mockCommonConfig = `version: "1"
delimiters: {left: "[[", right: "]]"}
default_tags: [operator]
inputs:
  type: object
  properties:
    pinned:
      type: string
      default: "1.0.2"
`

const mockSetConfig = `version: "1.0"
target: mocks/[[ .mock_id ]]
inputs:
  type: object
  additionalProperties: false
  required: [mock_id]
  properties:
    mock_id:
      description: The mock's id.
      type: string
      pattern: "^m[0-9]+$"
      tags: [requester]
    limits:
      type: object
      properties:
        size: {type: integer, default: 10}
        names: {type: array, items: {type: string}}
`

func mockTemplates(extraFiles map[string]string) fstest.MapFS {
	files := fstest.MapFS{
		"zerox/zerox.yml":          {Data: []byte(mockCommonConfig)},
		"zerox/mock-set/zerox.yml": {Data: []byte(mockSetConfig)},
	}
	for filePath, content := range extraFiles {
		files[filePath] = &fstest.MapFile{Data: []byte(content)}
	}
	return files
}

func readMockSet(testingContext *testing.T, files fstest.MapFS) TemplateSet {
	testingContext.Helper()
	set, err := ReadTemplateSet(files, "mock-set")
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

func TestASetReadsWithTheCommonConfigMergedUnderIt(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	if set.Name != "mock-set" || set.Delimiters != (Delimiters{Left: "[[", Right: "]]"}) || set.Target != "mocks/[[ .mock_id ]]" ||
		!reflect.DeepEqual(set.DefaultTags, []string{"operator"}) {
		testingContext.Fatalf("set: %+v", set)
	}
	var paths []string
	for _, input := range set.Inputs() {
		paths = append(paths, input.Path)
	}
	if expected := []string{"pinned", "mock_id", "limits", "limits.size", "limits.names"}; !reflect.DeepEqual(paths, expected) {
		testingContext.Fatalf("inputs %v, expected %v", paths, expected)
	}
}

func TestASetWithoutACommonConfigStatesEveryFieldItself(testingContext *testing.T) {
	files := mockTemplates(nil)
	delete(files, "zerox/zerox.yml")
	_, err := ReadTemplateSet(files, "mock-set")
	expectError(testingContext, err, "`delimiters` is stated by neither this file nor `zerox/zerox.yml`", "zerox/mock-set/zerox.yml")
	files["zerox/mock-set/zerox.yml"] = &fstest.MapFile{Data: []byte(mockSetConfig + "delimiters: {left: \"[[\", right: \"]]\"}\ndefault_tags: []\n")}
	set := readMockSet(testingContext, files)
	if len(set.DefaultTags) != 0 {
		testingContext.Fatalf("default tags %v", set.DefaultTags)
	}
}

func TestASetThatIsNotThereOrMisnamedIsRefused(testingContext *testing.T) {
	_, err := ReadTemplateSet(mockTemplates(nil), "missing-set")
	expectError(testingContext, err, "`missing-set` is not a template set: zerox/missing-set/zerox.yml is not in the templates read")
	_, err = ReadTemplateSet(mockTemplates(nil), "../mock-set")
	expectError(testingContext, err, "`../mock-set` is not a template set's name")
}

func TestAConfigFileIsHeldToItsVersionsSchema(testingContext *testing.T) {
	for config, expected := range map[string]string{
		"target: x\n":                     "`version` is required, quoted, one of `\"1\"` or `\"1.0\"`",
		"version: 1\n":                    "`version` is required, quoted",
		"version: \"2\"\n":                "`version` `\"2\"` is not a version zerox reads",
		"- a list\n":                      "a `zerox.yml` is a mapping of its fields",
		"version: \"1\"\nmystery: true\n": "mystery",
		"version: \"1\"\ndelimiters: {left: \"\", right: \"]]\"}\n":                        "left",
		"version: \"1\"\ninputs: {type: object, properties: {bad-name: {type: string}}}\n": "bad-name",
		"version: \"1\"\ninputs: {type: string, properties: {}}\n":                         "type",
		"version: \"1\"\ninputs: {type: object, properties: {m: {descripton: x}}}\n":       "descripton",
		"version: \"1\"\ninputs: {type: object, properties: {m: {minLength: -1}}}\n":       "minLength",
		"version: \"1\"\ntemplate: mock-set@v1\n":                                          "template set's `zerox.yml` states no `template`",
		"version: \"1\"\n  bad: [indent\n":                                                 "",
	} {
		files := mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": config})
		_, err := ReadTemplateSet(files, "mock-set")
		expectError(testingContext, err, expected)
	}
}

func TestAnOversizedConfigFileIsRefused(testingContext *testing.T) {
	files := mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": "version: \"1\"\n" + strings.Repeat("#", ConfigFileSizeLimit)})
	_, err := ReadTemplateSet(files, "mock-set")
	expectError(testingContext, err, "zerox/mock-set/zerox.yml is larger than 1 MiB")
}

func TestABrokenCommonConfigIsReportedAtItself(testingContext *testing.T) {
	files := mockTemplates(map[string]string{"zerox/zerox.yml": "version: \"1\"\nmystery: true\n"})
	_, err := ReadTemplateSet(files, "mock-set")
	expectError(testingContext, err, "zerox/zerox.yml", "mystery")
}

func TestInputsReadEveryJSONSchemaKeywordTheyHold(testingContext *testing.T) {
	config := `version: "1"
target: mocks/[[ .mock_id ]]
inputs:
  $comment: mock inputs
  type: object
  required: [mock_id]
  $defs:
    mock_name: {type: string, minLength: 1, maxLength: 20, format: hostname}
  properties:
    mock_id:
      title: Mock id
      description: The mock's id.
      type: [string, "null"]
      enum: [m1, m2]
      examples: [m1]
      deprecated: false
      tags: [requester]
    mock_name: {$ref: "#/$defs/mock_name"}
    mock_list:
      type: array
      prefixItems: [{const: first}]
      items: true
      contains: {type: string}
      minContains: 1
      uniqueItems: true
    mock_size:
      type: number
      multipleOf: 0.5
      exclusiveMinimum: 0
      maximum: 10
  if: {properties: {mock_id: {const: m1}}}
  then: {required: [mock_name]}
  unevaluatedProperties: false
`
	readMockSet(testingContext, mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": config}))
}

func TestInputsThatAreNotAJSONSchemaAreRefused(testingContext *testing.T) {
	files := mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": strings.Replace(mockSetConfig, `pattern: "^m[0-9]+$"`, `pattern: "(["`, 1)})
	_, err := ReadTemplateSet(files, "mock-set")
	expectError(testingContext, err, "the inputs of `mock-set` are not a JSON Schema")
}
