package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGitDiff_Match(t *testing.T) {
	f := GitDiff{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"diff"}, true},
		{"git", []string{"diff", "--staged"}, true},
		{"git", []string{"diff", "--cached"}, true},
		{"git", []string{"diff", "HEAD"}, true},
		{"git", []string{"diff", "HEAD~1", "HEAD"}, true},
		{"git", []string{"diff", "--", "file.go"}, true},
		{"git", []string{"diff", "--staged", "--", "src/"}, true},
		{"git", []string{"diff", "--no-color"}, true},
		// should not match
		{"git", []string{"status"}, false},
		{"git", []string{"diff", "--stat"}, false},
		{"git", []string{"diff", "--name-only"}, false},
		{"git", []string{"diff", "--word-diff"}, false},
		{"docker", []string{"diff"}, false},
		{"git", nil, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitDiff_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "clean", passthru: true},   // empty output — nothing to compress
		{name: "single_file"},
		{name: "modified_file"},
		{name: "multi_file"},
		{name: "new_file"},
		{name: "deleted_file"},
		{name: "renamed_file"},
		{name: "binary_file"},
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"diff"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_diff_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitDiff{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_diff_"+tt.name+".golden.txt", got)
		})
	}
}

func TestGitDiff_Passthrough(t *testing.T) {
	f := GitDiff{}
	ctx := ApplyContext{Cmd: "git", Args: []string{"diff"}}
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unknown_line",
			input: "diff --git a/f.go b/f.go\nsome unexpected line\n",
		},
		{
			name:  "ansi_color_codes",
			input: "diff --git a/f.go b/f.go\n\033[32m+added\033[m\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []byte(tt.input)
			got := f.Apply(in, ctx)
			if !bytes.Equal(got, in) {
				t.Fatalf("should pass through unchanged:\ngot:  %q\nwant: %q", got, in)
			}
		})
	}
}

func TestGitDiff_EmptyInputPassthrough(t *testing.T) {
	got := GitDiff{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestDefault_RegistersGitDiff(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"diff"})
	if f == nil {
		t.Fatal("Default() registry did not route `git diff`")
	}
	if f.Name() != "git-diff" {
		t.Fatalf("expected git-diff filter, got %q", f.Name())
	}
}
