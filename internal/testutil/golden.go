// Package testutil provides helpers for golden-file testing.
//
// Golden files live in the calling test package's testdata/ directory,
// which is the working directory when tests run. Update them with:
//
//	go test ./internal/filters/ -update
package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// Update is registered as a flag on import. When set, CheckGolden writes
// output instead of comparing.
var Update = flag.Bool("update", false, "overwrite golden files instead of comparing")

// CheckGolden compares got against testdata/<name> in the calling test's
// working directory. Pass -update to regenerate.
func CheckGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *Update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}
