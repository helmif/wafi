package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestJest_Match(t *testing.T) {
	f := Jest{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"jest", []string{}, true},
		{"jest", []string{"--watch"}, true},
		{"jest", []string{"--coverage"}, true},
		{"npx", []string{"jest"}, true},
		{"npx", []string{"jest", "--coverage"}, true},
		{"npx", []string{"vitest"}, false},
		{"npx", []string{}, false},
		{"vitest", []string{}, false},
		{"npm", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestJest_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "all_pass"},
		{name: "single_fail"},
		{name: "multi_fail"},
		{name: "only_fail", passthru: true},
	}
	ctx := ApplyContext{Cmd: "jest", Args: []string{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "jest_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := Jest{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "jest_"+tt.name+".golden.txt", got)
		})
	}
}
