package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheAssetsAreWrittenIntoTheFolderGiven(testingContext *testing.T) {
	folder := filepath.Join(testingContext.TempDir(), "assets")
	if err := writeAssets([]string{folder}); err != nil {
		testingContext.Fatal(err)
	}
	for _, name := range []string{"zerox-v1.schema.json", "zerox-v1.0.schema.json"} {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			testingContext.Fatal(err)
		}
	}
	if err := writeAssets(nil); err == nil {
		testingContext.Fatal("no folder was accepted")
	}
}
