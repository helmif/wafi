package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGitBranch_Match(t *testing.T) {
	f := GitBranch{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"branch"}, true},
		{"git", []string{"branch", "-a"}, true},
		{"git", []string{"branch", "--all"}, true},
		{"git", []string{"branch", "-r"}, true},
		{"git", []string{"branch", "--remotes"}, true},
		// should not match — mutations
		{"git", []string{"branch", "-d", "feat"}, false},
		{"git", []string{"branch", "--delete", "feat"}, false},
		{"git", []string{"branch", "-D", "feat"}, false},
		{"git", []string{"branch", "-m", "old", "new"}, false},
		{"git", []string{"branch", "--move", "old", "new"}, false},
		{"git", []string{"branch", "-u", "origin/main"}, false},
		{"git", []string{"branch", "--set-upstream-to", "origin/main"}, false},
		{"git", []string{"branch", "--unset-upstream"}, false},
		// should not match — verbose changes output shape
		{"git", []string{"branch", "-v"}, false},
		{"git", []string{"branch", "--verbose"}, false},
		{"git", []string{"branch", "-vv"}, false},
		// should not match — wrong cmd
		{"git", []string{"status"}, false},
		{"git", nil, false},
		{"docker", []string{"branch"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitBranch_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "local", passthru: true},
		{name: "all"},
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"branch"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_branch_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitBranch{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_branch_"+tt.name+".golden.txt", got)
		})
	}
}

func TestGitBranch_EmptyInput(t *testing.T) {
	got := GitBranch{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestGitBranch_DropsHEADPointer(t *testing.T) {
	input := "* main\n  remotes/origin/HEAD -> origin/main\n  remotes/origin/main\n"
	got := string(GitBranch{}.Apply([]byte(input), ApplyContext{}))
	if bytes.Contains([]byte(got), []byte("HEAD ->")) {
		t.Fatalf("HEAD pointer line should be dropped, got: %q", got)
	}
}

func TestGitBranch_ReformatsRemotes(t *testing.T) {
	input := "* main\n  remotes/origin/feature/auth\n"
	got := string(GitBranch{}.Apply([]byte(input), ApplyContext{}))
	want := "* main\n  [origin] feature/auth\n"
	if got != want {
		t.Fatalf("remotes reformatting wrong:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestDefault_RegistersGitBranch(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"branch"})
	if f == nil {
		t.Fatal("Default() registry did not route `git branch`")
	}
	if f.Name() != "git-branch" {
		t.Fatalf("expected git-branch filter, got %q", f.Name())
	}
}
