package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestGitStatus_Match(t *testing.T) {
	f := GitStatus{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"status"}, true},
		{"git", []string{"log"}, false},
		{"git", nil, false},
		{"git", []string{}, false},
		{"docker", []string{"status"}, false},
		{"git", []string{"status", "--short"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitStatus_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "clean"},
		{name: "dirty"},
		{name: "staged"},
		{name: "ahead"},
		{name: "behind"},
		{name: "diverged"},
		{name: "detached"},
		{name: "conflict"},
		{name: "malformed", passthru: true},
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"status"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_status_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitStatus{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_status_"+tt.name+".golden.txt", got)
		})
	}
}

// TestGitStatus_Passthrough covers the bail-out paths inside compressGitStatus
// that return (nil, false) → Apply returns raw input unchanged.
func TestGitStatus_Passthrough(t *testing.T) {
	f := GitStatus{}
	ctx := ApplyContext{Cmd: "git", Args: []string{"status"}}
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unknown_your_branch_message",
			input: "On branch main\nYour branch is somewhere over the rainbow.\n",
		},
		{
			name:  "tab_indented_outside_section",
			input: "On branch main\n\tsome line outside any section\n",
		},
		{
			name:  "duplicate_branch_header",
			input: "On branch main\nOn branch other\n",
		},
		{
			name:  "diverged_missing_tail",
			input: "On branch main\nYour branch and 'origin/main' have diverged,\n",
		},
		{
			name:  "diverged_bad_tail",
			input: "On branch main\nYour branch and 'origin/main' have diverged,\nnot the expected format\n",
		},
		{
			name:  "entry_no_colon",
			input: "On branch main\n\nChanges to be committed:\n\tnocolon\n",
		},
		{
			name:  "entry_unknown_label",
			input: "On branch main\n\nChanges to be committed:\n\tunknown:   foo.go\n",
		},
		{
			name:  "unmerged_no_colon",
			input: "On branch main\n\nUnmerged paths:\n\tnocolon\n",
		},
		{
			name:  "unmerged_unknown_label",
			input: "On branch main\n\nUnmerged paths:\n\tunknown:   foo.go\n",
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
