package zerox

import (
	"encoding/json"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestAConfigStatesAMajorOrAMinor(testingContext *testing.T) {
	if actual := StatedVersions(); !reflect.DeepEqual(actual, []string{"1", "1.0"}) {
		testingContext.Fatalf("stated versions %v", actual)
	}
	for stated, expected := range map[string]string{"1": "1.0", "1.0": "1.0"} {
		text, minorVersion, err := SchemaFor(stated)
		if err != nil || minorVersion != expected || !json.Valid(text) {
			testingContext.Fatalf("%s: %s %v", stated, minorVersion, err)
		}
	}
	_, _, err := SchemaFor("2")
	expectError(testingContext, err, `"2" is not a version zerox reads`)
}

func TestAReleaseAttachesEveryMinorAndEachMajorAsItsNewestMinor(testingContext *testing.T) {
	assets, err := ReleaseAssets()
	if err != nil {
		testingContext.Fatal(err)
	}
	if len(assets) != 2 || !reflect.DeepEqual(assets["zerox-v1.schema.json"], assets["zerox-v1.0.schema.json"]) {
		testingContext.Fatalf("assets %v", len(assets))
	}
}

func TestTheSchemaFilesAreNamedByTheirVersionAndOrderedByIt(testingContext *testing.T) {
	read, err := readMinorSchemas(fstest.MapFS{
		"schemas/zerox-v1.10.schema.json": {}, "schemas/zerox-v1.2.schema.json": {}, "schemas/zerox-v2.0.schema.json": {},
	})
	if err != nil {
		testingContext.Fatal(err)
	}
	var versions []string
	for _, schema := range read {
		versions = append(versions, schema.version())
	}
	if !reflect.DeepEqual(versions, []string{"1.2", "1.10", "2.0"}) {
		testingContext.Fatalf("versions %v", versions)
	}
	_, err = readMinorSchemas(fstest.MapFS{"schemas/mystery.json": {}})
	expectError(testingContext, err, "mystery.json is not named zerox-v<major>.<minor>.schema.json")
	_, err = readMinorSchemas(fstest.MapFS{})
	expectError(testingContext, err, "failed to list the embedded schemas")
}
