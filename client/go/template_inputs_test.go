package zerox

import (
	"reflect"
	"testing"
)

func TestAnInputTakesItsOwnTagsElseTheDefaultTagsAndItsSchemasDefault(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	inputs := map[string]Input{}
	for _, input := range set.Inputs() {
		inputs[input.Path] = input
	}
	if mockID := inputs["mock_id"]; !reflect.DeepEqual(mockID.Tags, []string{"requester"}) || mockID.HasDefault ||
		mockID.Schema["description"] != "The mock's id." {
		testingContext.Fatalf("mock_id: %+v", mockID)
	}
	if size := inputs["limits.size"]; !reflect.DeepEqual(size.Tags, []string{"operator"}) || !size.HasDefault || size.Default != int64(10) {
		testingContext.Fatalf("limits.size: %+v", size)
	}
	expectedTags := map[string][]string{
		"pinned": {"operator"}, "mock_id": {"requester"}, "limits": {"operator"}, "limits.size": {"operator"}, "limits.names": {"operator"},
	}
	if actual := set.InputTags(); !reflect.DeepEqual(actual, expectedTags) {
		testingContext.Fatalf("input tags %v", actual)
	}
}

func TestValuesLeftOutTakeTheirDefaultsAtAnyDepthAndGivenOnesStay(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	given := map[string]any{"mock_id": "m1", "limits": map[string]any{"names": []any{"a"}}, "extra": nil}
	completed := set.CompletedValues(given)
	expected := map[string]any{"mock_id": "m1", "pinned": "1.0.2", "limits": map[string]any{"names": []any{"a"}, "size": int64(10)}, "extra": nil}
	if !reflect.DeepEqual(completed, expected) {
		testingContext.Fatalf("completed %v, expected %v", completed, expected)
	}
	if _, isAdded := given["pinned"]; isAdded {
		testingContext.Fatal("completing changed the values given")
	}
	if completed := set.CompletedValues(nil); !reflect.DeepEqual(completed, map[string]any{"pinned": "1.0.2", "limits": map[string]any{"size": int64(10)}}) {
		testingContext.Fatalf("completed nothing given: %v", completed)
	}
}

func TestValuesAreHeldToTheInputsSchema(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	if err := set.CheckValues(set.CompletedValues(map[string]any{"mock_id": "m1"})); err != nil {
		testingContext.Fatal(err)
	}
	err := set.CheckValues(set.CompletedValues(map[string]any{"mock_id": "bad", "mystery": true}))
	expectError(testingContext, err, "the values", "`mock_id` must match pattern", "mystery", "2 errors")
	expectError(testingContext, set.CheckValues(map[string]any{"mock_id": "m1", "limits": map[string]any{"size": "x"}}), "1 error")
}

func TestValuesThatYAMLCannotWriteAreReported(testingContext *testing.T) {
	set := readMockSet(testingContext, mockTemplates(nil))
	expectError(testingContext, set.CheckValues(map[string]any{"mock_id": func() {}}), "failed to write the values as YAML")
}
