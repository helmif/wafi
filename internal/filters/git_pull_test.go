package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGitPull_Match(t *testing.T) {
	f := GitPull{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"pull"}, true},
		{"git", []string{"pull", "origin"}, true},
		{"git", []string{"pull", "origin", "main"}, true},
		{"git", []string{"pull", "--ff-only"}, true},
		{"git", []string{"pull", "--no-rebase"}, true},
		// should not match
		{"git", []string{"pull", "--rebase"}, false},
		{"git", []string{"pull", "-r"}, false},
		{"git", []string{"push"}, false},
		{"git", nil, false},
		{"docker", []string{"pull"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitPull_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "up_to_date", passthru: true},
		{name: "fast_forward"},
		{name: "merge"},
		{name: "conflict"},
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"pull"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_pull_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitPull{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_pull_"+tt.name+".golden.txt", got)
		})
	}
}

func TestGitPull_EmptyInput(t *testing.T) {
	got := GitPull{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestDefault_RegistersGitPull(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"pull"})
	if f == nil {
		t.Fatal("Default() registry did not route `git pull`")
	}
	if f.Name() != "git-pull" {
		t.Fatalf("expected git-pull filter, got %q", f.Name())
	}
}
