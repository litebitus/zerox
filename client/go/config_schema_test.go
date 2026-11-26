package zerox

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

var jsonSchemaKeywords = []string{
	"$anchor", "$comment", "$defs", "$dynamicAnchor", "$dynamicRef", "$id", "$ref", "$schema", "$vocabulary",
	"additionalProperties", "allOf", "anyOf", "contains", "dependentSchemas", "else", "if", "items", "not",
	"oneOf", "patternProperties", "prefixItems", "properties", "propertyNames", "then",
	"unevaluatedItems", "unevaluatedProperties",
	"const", "dependentRequired", "enum", "exclusiveMaximum", "exclusiveMinimum", "maxContains", "maximum",
	"maxItems", "maxLength", "maxProperties", "minContains", "minimum", "minItems", "minLength",
	"minProperties", "multipleOf", "pattern", "required", "type", "uniqueItems",
	"default", "deprecated", "description", "examples", "readOnly", "title", "writeOnly",
	"format",
	"contentEncoding", "contentMediaType", "contentSchema",
}

var propertiesDescribedThroughAllOf = []string{
	"/$defs/templates_config/properties/inputs/properties/$schema",
	"/$defs/templates_config/properties/inputs/properties/type",
}

func newestSchema(testingContext *testing.T) map[string]any {
	testingContext.Helper()
	text, _, err := SchemaFor("1")
	if err != nil {
		testingContext.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(text, &schema); err != nil {
		testingContext.Fatal(err)
	}
	return schema
}

func undescribedProperties(schema map[string]any, path string) []string {
	var undescribed []string
	for _, mapKeyword := range []string{"properties", "$defs"} {
		children, _ := schema[mapKeyword].(map[string]any)
		for name, child := range children {
			childPath := path + "/" + mapKeyword + "/" + name
			childSchema, isSchema := child.(map[string]any)
			if !isSchema {
				continue
			}
			if _, described := childSchema["description"]; mapKeyword == "properties" && !described &&
				len(childSchema) > 0 && !slices.Contains(propertiesDescribedThroughAllOf, childPath) {
				undescribed = append(undescribed, childPath)
			}
			undescribed = append(undescribed, undescribedProperties(childSchema, childPath)...)
		}
	}
	for _, listKeyword := range []string{"allOf", "anyOf", "oneOf"} {
		children, _ := schema[listKeyword].([]any)
		for index, child := range children {
			if childSchema, isSchema := child.(map[string]any); isSchema {
				undescribed = append(undescribed, undescribedProperties(childSchema, fmt.Sprintf("%s/%s/%d", path, listKeyword, index))...)
			}
		}
	}
	for _, schemaKeyword := range []string{"if", "then", "else", "additionalProperties", "items"} {
		if childSchema, isSchema := schema[schemaKeyword].(map[string]any); isSchema {
			undescribed = append(undescribed, undescribedProperties(childSchema, path+"/"+schemaKeyword)...)
		}
	}
	return undescribed
}

func TestTheInputSchemaDefinesEveryJSONSchemaKeywordAndTags(testingContext *testing.T) {
	inputSchema := newestSchema(testingContext)["$defs"].(map[string]any)["input_schema"].(map[string]any)
	defined := inputSchema["properties"].(map[string]any)
	expected := append(slices.Clone(jsonSchemaKeywords), "tags")
	for _, keyword := range expected {
		if _, isDefined := defined[keyword]; !isDefined {
			testingContext.Errorf("the input schema defines no `%s`", keyword)
		}
	}
	for keyword := range defined {
		if !slices.Contains(expected, keyword) {
			testingContext.Errorf("the input schema defines `%s`, which is neither JSON Schema's nor `tags`", keyword)
		}
	}
	if inputSchema["additionalProperties"] != false {
		testingContext.Error("the input schema admits keys it does not define")
	}
}

func TestEveryPropertyTheSchemaDefinesIsDescribed(testingContext *testing.T) {
	if undescribed := undescribedProperties(newestSchema(testingContext), ""); len(undescribed) > 0 {
		slices.Sort(undescribed)
		testingContext.Fatalf("properties without a description:\n%s", strings.Join(undescribed, "\n"))
	}
}

func inputsConfig(input string) string {
	return "version: \"1\"\ntarget: mocks/[[ .mock_id ]]\ninputs:\n  type: object\n  properties:\n    mock_id: " + input + "\n"
}

func TestAnInputHoldingEachKeywordAsItsDefinitionStatesIsRead(testingContext *testing.T) {
	for _, input := range []string{
		`{type: string, title: Mock, description: A mock., default: m1, examples: [m1], deprecated: true}`,
		`{type: [string, "null"], enum: [m1, null], const: m1}`,
		`{type: string, minLength: 1, maxLength: 9, pattern: "^m", format: hostname}`,
		`{type: number, minimum: 0, maximum: 9, exclusiveMinimum: -1, exclusiveMaximum: 10, multipleOf: 0.5}`,
		`{type: array, prefixItems: [{const: m1}], items: false, contains: true, minContains: 1, maxContains: 2}`,
		`{type: array, minItems: 1, maxItems: 3, uniqueItems: true, unevaluatedItems: false}`,
		`{type: object, properties: {inner: {type: string}}, patternProperties: {"^m": true}, ` +
			`additionalProperties: false, propertyNames: {pattern: "^[a-z]"}, minProperties: 1, maxProperties: 4}`,
		`{type: object, required: [inner], dependentRequired: {inner: [other]}, ` +
			`dependentSchemas: {inner: {required: [other]}}, unevaluatedProperties: false}`,
		`{allOf: [true], anyOf: [{type: string}], oneOf: [{type: string}], not: {type: integer}}`,
		`{if: {type: string}, then: {minLength: 1}, else: {type: integer}}`,
		`{$anchor: mock, $dynamicAnchor: mock, $comment: mock, $defs: {inner: true}, ` +
			`$ref: "#/properties/mock_id/$defs/inner"}`,
		`{$schema: "https://json-schema.org/draft/2020-12/schema", $id: "https://mock.example/m", ` +
			`$vocabulary: {"https://mock.example/vocabulary": true}, ` +
			`$defs: {node: {$dynamicAnchor: mock, type: string}}, $dynamicRef: "#mock"}`,
		`{type: string, contentEncoding: base64, contentMediaType: application/json, contentSchema: {type: object}}`,
		`{type: string, readOnly: true, writeOnly: false, tags: [requester]}`,
	} {
		files := mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": inputsConfig(input)})
		if _, err := ReadTemplateSet(files, "mock-set"); err != nil {
			testingContext.Errorf("%s refused: %v", input, err)
		}
	}
}

func TestAnInputHoldingAKeywordAgainstItsDefinitionIsRefused(testingContext *testing.T) {
	for input, expected := range map[string]string{
		`{type: text}`:                   "type",
		`{type: [string, string]}`:       "type",
		`{enum: []}`:                     "enum",
		`{minLength: -1}`:                "minLength",
		`{maxItems: 1.5}`:                "maxItems",
		`{multipleOf: 0}`:                "multipleOf",
		`{uniqueItems: yes}`:             "uniqueItems",
		`{required: [inner, inner]}`:     "required",
		`{allOf: []}`:                    "allOf",
		`{items: 3}`:                     "items",
		`{$anchor: "1mock"}`:             "$anchor",
		`{description: [a, list]}`:       "description",
		`{tags: [requester, requester]}`: "tags",
		`{descripton: A typo.}`:          "descripton",
		`{properties: {bad-name: true}}`: "bad-name",
	} {
		files := mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": inputsConfig(input)})
		_, err := ReadTemplateSet(files, "mock-set")
		expectError(testingContext, err, expected)
	}
}

const draftTwentyTwentyInputs = `version: "1"
target: mocks/[[ .mock_id ]]
inputs:
  type: object
  dependentRequired: {mock_id: [mock_name]}
  properties:
    mock_id: {type: string}
    mock_name: {type: string}
    mock_list: {type: array, prefixItems: [{const: first}], items: false}
`

func TestTheInputsAreReadInTheDraftTheSchemaStates(testingContext *testing.T) {
	inputs := newestSchema(testingContext)["$defs"].(map[string]any)["templates_config"].(map[string]any)["properties"].(map[string]any)["inputs"].(map[string]any)
	stated := inputs["properties"].(map[string]any)["$schema"].(map[string]any)["const"]
	if stated != inputsSchemaDraft {
		testingContext.Fatalf("the schema holds the inputs to %v, and they are read as %s", stated, inputsSchemaDraft)
	}
}

func TestValuesAreHeldToAKeywordOnlyTheInputsDraftHas(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": draftTwentyTwentyInputs}))
	accepted := map[string]any{"mock_id": "m1", "mock_name": "mock", "mock_list": []any{"first"}}
	if err := set.CheckValues(accepted); err != nil {
		testingContext.Fatal(err)
	}
	expectError(testingContext, set.CheckValues(map[string]any{"mock_id": "m1"}), "mock_name")
	expectError(testingContext, set.CheckValues(map[string]any{"mock_list": []any{"first", "second"}}), "mock_list")
}

func TestInputsStatingTheirDraftStateTheOneTheyAreReadIn(testingContext *testing.T) {
	stating := func(draft string) fstest.MapFS {
		config := strings.Replace(draftTwentyTwentyInputs, "  type: object\n", "  $schema: "+draft+"\n  type: object\n", 1)
		return mockTemplates(map[string]string{"zerox/mock-set/zerox.yml": config})
	}
	readMockSet(testingContext, stating(inputsSchemaDraft))
	_, err := ReadTemplateSet(stating("http://json-schema.org/draft-07/schema#"), "mock-set")
	expectError(testingContext, err, "$schema")
}
