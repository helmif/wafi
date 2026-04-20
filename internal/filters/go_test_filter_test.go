package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGoTest_Match(t *testing.T) {
	f := GoTest{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"go", []string{"test"}, true},
		{"go", []string{"test", "./..."}, true},
		{"go", []string{"test", "-run", "TestFoo"}, true},
		{"go", []string{"test", "-count=1", "./..."}, true},
		{"go", []string{"test", "-v"}, false},           // verbose: passthrough
		{"go", []string{"test", "-v", "./..."}, false},  // verbose with pkg
		{"go", []string{"build"}, false},
		{"go", nil, false},
		{"go", []string{}, false},
		{"python", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGoTest_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "all_pass"},
		{name: "single_fail"},
		{name: "multi_fail"},
		{name: "with_coverage"},
		{name: "panic", passthru: true},
		{name: "no_test_files", passthru: true},
	}
	ctx := ApplyContext{Cmd: "go", Args: []string{"test", "./..."}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "go_test_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GoTest{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "go_test_"+tt.name+".golden.txt", got)
		})
	}
}
