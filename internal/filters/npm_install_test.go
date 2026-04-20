package filters

import (
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestNpmInstall_Match(t *testing.T) {
	f := NpmInstall{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"npm", []string{"install"}, true},
		{"npm", []string{"i"}, true},
		{"npm", []string{"ci"}, true},
		{"npm", []string{"add", "express"}, true},
		{"npm", []string{"remove", "lodash"}, true},
		{"npm", []string{"uninstall", "lodash"}, true},
		{"npm", []string{"update"}, true},
		{"npm", []string{"install", "--save-dev", "jest"}, true},
		// should not match
		{"npm", []string{"run", "build"}, false},
		{"npm", []string{"publish"}, false},
		{"npm", nil, false},
		{"node", []string{"install"}, false},
		{"pnpm", []string{"install"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestNpmInstall_Golden(t *testing.T) {
	tests := []string{"success", "vulns", "deprecated", "error"}
	ctx := ApplyContext{Cmd: "npm", Args: []string{"install"}}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "npm_install_"+name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := NpmInstall{}.Apply(input, ctx)
			testutil.CheckGolden(t, "npm_install_"+name+".golden.txt", got)
		})
	}
}

func TestNpmInstall_EmptyInput(t *testing.T) {
	got := NpmInstall{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestNpmInstall_AllDropped_ReturnsRaw(t *testing.T) {
	// Input that consists entirely of lines that will be dropped.
	input := []byte("32 packages are looking for funding\n  run `npm fund` for details\n")
	got := NpmInstall{}.Apply(input, ApplyContext{})
	if string(got) != string(input) {
		t.Fatalf("all-dropped input should return raw:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestDefault_RegistersNpmInstall(t *testing.T) {
	r := Default()
	f := r.Lookup("npm", []string{"install"})
	if f == nil {
		t.Fatal("Default() registry did not route `npm install`")
	}
	if f.Name() != "npm-install" {
		t.Fatalf("expected npm-install filter, got %q", f.Name())
	}
}
