package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGitPush_Match(t *testing.T) {
	f := GitPush{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"push"}, true},
		{"git", []string{"push", "origin", "main"}, true},
		{"git", []string{"push", "--force"}, true},
		{"git", []string{"push", "--tags"}, true},
		{"git", []string{"push", "--force-with-lease"}, true},
		{"git", []string{"push", "-u", "origin", "main"}, true},
		// should not match
		{"git", []string{"push", "--porcelain"}, false},
		{"git", []string{"pull"}, false},
		{"git", nil, false},
		{"docker", []string{"push"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitPush_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "success"},
		{name: "up_to_date", passthru: true},
		{name: "rejected"},
		{name: "new_branch"},
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"push"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_push_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitPush{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_push_"+tt.name+".golden.txt", got)
		})
	}
}

func TestGitPush_EmptyInput(t *testing.T) {
	got := GitPush{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestDefault_RegistersGitPush(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"push"})
	if f == nil {
		t.Fatal("Default() registry did not route `git push`")
	}
	if f.Name() != "git-push" {
		t.Fatalf("expected git-push filter, got %q", f.Name())
	}
}
