package filters

import (
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestPnpmInstall_Match(t *testing.T) {
	f := PnpmInstall{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"pnpm", []string{"install"}, true},
		{"pnpm", []string{"i"}, true},
		{"pnpm", []string{"add", "express"}, true},
		{"pnpm", []string{"remove", "lodash"}, true},
		{"pnpm", []string{"update"}, true},
		{"pnpm", []string{"up"}, true},
		{"pnpm", nil, true}, // bare pnpm runs install
		// should not match
		{"pnpm", []string{"run", "build"}, false},
		{"pnpm", []string{"exec", "jest"}, false},
		{"npm", []string{"install"}, false},
		{"yarn", []string{"install"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestPnpmInstall_Golden(t *testing.T) {
	tests := []string{"success", "with_deps", "error"}
	ctx := ApplyContext{Cmd: "pnpm", Args: []string{"install"}}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "pnpm_install_"+name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := PnpmInstall{}.Apply(input, ctx)
			testutil.CheckGolden(t, "pnpm_install_"+name+".golden.txt", got)
		})
	}
}

func TestPnpmInstall_EmptyInput(t *testing.T) {
	got := PnpmInstall{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestPnpmInstall_AllDropped_ReturnsRaw(t *testing.T) {
	// Input that consists only of plus-progress lines (all dropped).
	input := []byte("++++++++++++++++++++++++\n")
	got := PnpmInstall{}.Apply(input, ApplyContext{})
	if string(got) != string(input) {
		t.Fatalf("all-dropped input should return raw:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestDefault_RegistersPnpmInstall(t *testing.T) {
	r := Default()
	f := r.Lookup("pnpm", []string{"install"})
	if f == nil {
		t.Fatal("Default() registry did not route `pnpm install`")
	}
	if f.Name() != "pnpm-install" {
		t.Fatalf("expected pnpm-install filter, got %q", f.Name())
	}
}
