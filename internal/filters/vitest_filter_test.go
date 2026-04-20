package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestVitest_Match(t *testing.T) {
	f := Vitest{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"vitest", []string{}, true},
		{"vitest", []string{"run"}, true},
		{"vitest", []string{"--reporter=verbose"}, true},
		{"npx", []string{"vitest"}, true},
		{"npx", []string{"vitest", "run"}, true},
		{"npx", []string{"jest"}, false},
		{"npx", []string{}, false},
		{"jest", []string{}, false},
		{"npm", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestVitest_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "all_pass"},
		{name: "single_fail"},
		{name: "only_fail", passthru: true},
	}
	ctx := ApplyContext{Cmd: "vitest", Args: []string{"run"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "vitest_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := Vitest{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "vitest_"+tt.name+".golden.txt", got)
		})
	}
}
