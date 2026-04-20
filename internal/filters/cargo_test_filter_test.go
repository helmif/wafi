package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestCargoTest_Match(t *testing.T) {
	f := CargoTest{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"cargo", []string{"test"}, true},
		{"cargo", []string{"test", "--release"}, true},
		{"cargo", []string{"test", "my_test_fn"}, true},
		{"cargo", []string{"build"}, false},
		{"cargo", []string{"run"}, false},
		{"cargo", []string{}, false},
		{"go", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestCargoTest_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "all_pass"},
		{name: "single_fail"},
		{name: "multi_fail"},
		{name: "no_tests"},
		{name: "build_error", passthru: true},
	}
	ctx := ApplyContext{Cmd: "cargo", Args: []string{"test"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "cargo_test_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := CargoTest{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "cargo_test_"+tt.name+".golden.txt", got)
		})
	}
}
