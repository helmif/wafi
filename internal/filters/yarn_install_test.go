package filters

import (
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestYarnInstall_Match(t *testing.T) {
	f := YarnInstall{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"yarn", []string{"install"}, true},
		{"yarn", []string{"add", "express"}, true},
		{"yarn", []string{"remove", "lodash"}, true},
		{"yarn", []string{"upgrade"}, true},
		{"yarn", []string{"up"}, true},
		{"yarn", nil, true}, // bare `yarn` runs install
		// should not match
		{"yarn", []string{"run", "build"}, false},
		{"yarn", []string{"workspace", "app", "add", "react"}, false},
		{"npm", []string{"install"}, false},
		{"pnpm", []string{"add"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestYarnInstall_Golden(t *testing.T) {
	tests := []string{"classic", "berry", "error"}
	ctx := ApplyContext{Cmd: "yarn", Args: []string{"add"}}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "yarn_install_"+name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := YarnInstall{}.Apply(input, ctx)
			testutil.CheckGolden(t, "yarn_install_"+name+".golden.txt", got)
		})
	}
}

func TestYarnInstall_EmptyInput(t *testing.T) {
	got := YarnInstall{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestYarnInstall_AllDropped_ReturnsRaw(t *testing.T) {
	// Input that consists only of step-progress lines (all dropped).
	input := []byte("[1/4] Resolving packages...\n[2/4] Fetching packages...\n")
	got := YarnInstall{}.Apply(input, ApplyContext{})
	if string(got) != string(input) {
		t.Fatalf("all-dropped input should return raw:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestDefault_RegistersYarnInstall(t *testing.T) {
	r := Default()
	f := r.Lookup("yarn", []string{"install"})
	if f == nil {
		t.Fatal("Default() registry did not route `yarn install`")
	}
	if f.Name() != "yarn-install" {
		t.Fatalf("expected yarn-install filter, got %q", f.Name())
	}
}
