package zerox

import (
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

func TestALineageReadsAsItIsWritten(testingContext *testing.T) {
	for text, expected := range map[string]Lineage{
		"mock-set@4e436dc":               {Set: "mock-set", Ref: "4e436dc"},
		"mock.set@main":                  {Set: "mock.set", Ref: "main"},
		"mock_set@mock-templates/v1.2.3": {Set: "mock_set", Ref: "mock-templates/v1.2.3"},
		"mock-set@main@{1}":              {Set: "mock-set", Ref: "main@{1}"},
	} {
		lineage, err := ParseLineage(text)
		if err != nil || lineage != expected || lineage.String() != text {
			testingContext.Fatalf("%s read as %+v: %v", text, lineage, err)
		}
	}
	for _, text := range []string{"mock-set", "mock-set@", "@v1", "mock-set@-v1", "mock-set@v 1", "mock set@v1"} {
		_, err := ParseLineage(text)
		expectError(testingContext, err, "`"+text+"` is not a lineage")
	}
}

func TestATargetsConfigReadsItsLineageValuesTagsAndFingerprints(testingContext *testing.T) {
	text := `version: "1"
template: mock-set@v1
generated_at: "2026-11-25T08:00:00Z"
values:
  mock_id: m1
  limits: {size: 10}
input_tags:
  mock_id: [requester]
fingerprints:
  mock.yml: 2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae
`
	read, err := ReadTarget(fstest.MapFS{"mocks/m1/zerox.yml": {Data: []byte(text)}}, "mocks/m1/zerox.yml")
	if err != nil {
		testingContext.Fatal(err)
	}
	expected := Target{
		Lineage:      Lineage{Set: "mock-set", Ref: "v1"},
		GeneratedAt:  time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC),
		Values:       map[string]any{"mock_id": "m1", "limits": map[string]any{"size": int64(10)}},
		InputTags:    map[string][]string{"mock_id": {"requester"}},
		Fingerprints: map[string]string{"mock.yml": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
	}
	if !reflect.DeepEqual(read, expected) {
		testingContext.Fatalf("read %+v", read)
	}
}

func TestATargetsConfigIsHeldToItsSchemaAndStatesItsLineage(testingContext *testing.T) {
	complete := "version: \"1\"\ntemplate: mock-set@v1\ngenerated_at: \"2026-11-25T08:00:00Z\"\nvalues: {}\ninput_tags: {}\nfingerprints: {}\n"
	for text, expected := range map[string]string{
		"version: \"1\"\ntarget: mocks\n":         "a target's `zerox.yml` states `template`, its lineage",
		"version: \"1\"\ntemplate: mock-set@v1\n": "generated_at",
		complete + "mystery: 1\n":                 "mystery",
		"version: \"1\"\ntemplate: mock-set@v1\ngenerated_at: \"2026-13-25T08:00:00Z\"\nvalues: {}\ninput_tags: {}\nfingerprints: {}\n": "`generated_at` `2026-13-25T08:00:00Z` is not a time",
	} {
		_, err := ReadTarget(fstest.MapFS{"zerox.yml": {Data: []byte(text)}}, "zerox.yml")
		expectError(testingContext, err, expected)
	}
	_, err := ReadTarget(fstest.MapFS{}, "zerox.yml")
	expectError(testingContext, err, "failed to read zerox.yml")
}

func TestATargetsConfigWithALineageRefStartingWithADashIsRefused(testingContext *testing.T) {
	text := "version: \"1\"\ntemplate: mock-set@-v1\ngenerated_at: \"2026-11-25T08:00:00Z\"\nvalues: {}\ninput_tags: {}\nfingerprints: {}\n"
	_, err := ReadTarget(fstest.MapFS{"zerox.yml": {Data: []byte(text)}}, "zerox.yml")
	expectError(testingContext, err, "template")
}
