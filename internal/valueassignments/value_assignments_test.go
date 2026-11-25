package valueassignments

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestAnAssignmentReadsItsPathAndItsValueAsYAML(testingContext *testing.T) {
	for assignment, expected := range map[string]any{
		"limits.size=10":   int64(10),
		"mock_id=m1":       "m1",
		"version=\"1.0\"":  "1.0",
		"limits={size: 3}": map[string]any{"size": int64(3)},
		"names=[a, b]":     []any{"a", "b"},
		"empty=":           nil,
		"text=a=b":         "a=b",
	} {
		_, value, err := ParseValueAssignment(assignment)
		if err != nil || !reflect.DeepEqual(value, expected) {
			testingContext.Fatalf("%s read as %#v: %v", assignment, value, err)
		}
	}
	_, _, err := ParseValueAssignment("no-equals")
	expectError(testingContext, err, "`no-equals` is not `<path>=<value>`")
	_, _, err = ParseValueAssignment("bad-name=1")
	expectError(testingContext, err, "`bad-name` is not an input's path")
	_, _, err = ParseValueAssignment("a..b=1")
	expectError(testingContext, err, "`a..b` is not an input's path")
	_, _, err = ParseValueAssignment("a=[unclosed")
	expectError(testingContext, err, "the value of `a` is not YAML")
}

func TestSettingAValueMakesEachMapOnTheWay(testingContext *testing.T) {
	values := map[string]any{"limits": nil, "text": "x"}
	if err := SetValue(values, "limits.size", int64(3)); err != nil {
		testingContext.Fatal(err)
	}
	if err := SetValue(values, "deep.down.name", "n"); err != nil {
		testingContext.Fatal(err)
	}
	if err := SetValue(values, "limits.names", []any{"a"}); err != nil {
		testingContext.Fatal(err)
	}
	expected := map[string]any{
		"limits": map[string]any{"size": int64(3), "names": []any{"a"}},
		"deep":   map[string]any{"down": map[string]any{"name": "n"}},
		"text":   "x",
	}
	if !reflect.DeepEqual(values, expected) {
		testingContext.Fatalf("values %v", values)
	}
	expectError(testingContext, SetValue(values, "text.inner", 1), "`text` holds a value that is not a map, so `text.inner` cannot be set")
	expectError(testingContext, SetValue(values, "bad-name", 1), "is not an input's path")
}
