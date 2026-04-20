package filters

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wafi/internal/testutil"
)

func TestGitLog_Match(t *testing.T) {
	f := GitLog{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"git", []string{"log"}, true},
		{"git", []string{"log", "--stat"}, true},
		{"git", []string{"log", "HEAD~3"}, true},
		{"git", []string{"log", "HEAD~1", "HEAD"}, true},
		{"git", []string{"log", "--no-color"}, true},
		{"git", []string{"log", "--stat", "HEAD~2.."}, true},
		{"git", []string{"log", "--"}, true},
		// should not match
		{"git", []string{"log", "--oneline"}, false},
		{"git", []string{"log", "--format=%h %s"}, false},
		{"git", []string{"log", "--pretty=oneline"}, false},
		{"git", []string{"log", "--word-diff"}, false},
		{"git", []string{"diff"}, false},
		{"git", nil, false},
		{"docker", []string{"log"}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestGitLog_Golden(t *testing.T) {
	tests := []struct {
		name     string
		passthru bool
	}{
		{name: "simple"},
		{name: "verbose"},
		{name: "with_stats"},
		{name: "merge_commits"},
		{name: "oneline", passthru: true}, // --oneline not matched → pass-through
	}
	ctx := ApplyContext{Cmd: "git", Args: []string{"log"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "git_log_"+tt.name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := GitLog{}.Apply(input, ctx)
			if tt.passthru {
				if !bytes.Equal(got, input) {
					t.Fatalf("pass-through broken:\ngot:  %q\nwant: %q", got, input)
				}
				return
			}
			testutil.CheckGolden(t, "git_log_"+tt.name+".golden.txt", got)
		})
	}
}

func TestGitLog_Passthrough(t *testing.T) {
	f := GitLog{}
	ctx := ApplyContext{Cmd: "git", Args: []string{"log"}}
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unknown_header_line",
			input: "commit abc1234\nAuthor: Name <e@x.com>\nUnknown: field\nDate:   Mon Apr 20 12:00:00 2026 +0000\n\n    subject\n",
		},
		{
			name:  "content_before_first_commit",
			input: "warning: something\ncommit abc1234\nAuthor: Name <e@x.com>\n",
		},
		{
			name:  "hash_too_short",
			input: "commit abc\nAuthor: Name <e@x.com>\nDate:   Mon Apr 20 12:00:00 2026 +0000\n\n    subject\n",
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

func TestGitLog_EmptyInput(t *testing.T) {
	got := GitLog{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestGitLogRelDate(t *testing.T) {
	anchor := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		offset time.Duration
		want   string
	}{
		{30 * time.Second, "just now"},
		{90 * time.Second, "just now"},
		{5 * time.Minute, "5 minutes ago"},
		{59 * time.Minute, "59 minutes ago"},
		{65 * time.Minute, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{25 * time.Hour, "1 day ago"},
		{3 * 24 * time.Hour, "3 days ago"},
		{8 * 24 * time.Hour, "1 week ago"},
		{18 * 24 * time.Hour, "2 weeks ago"},
		{35 * 24 * time.Hour, "1 month ago"},
		{90 * 24 * time.Hour, "3 months ago"},
		{380 * 24 * time.Hour, "1 year ago"},
		{800 * 24 * time.Hour, "2 years ago"},
	}
	for _, tc := range tests {
		got := gitLogRelDate(anchor.Add(-tc.offset))
		if got != tc.want {
			t.Errorf("gitLogRelDate(now - %v) = %q, want %q", tc.offset, got, tc.want)
		}
	}
}

func TestDefault_RegistersGitLog(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"log"})
	if f == nil {
		t.Fatal("Default() registry did not route `git log`")
	}
	if f.Name() != "git-log" {
		t.Fatalf("expected git-log filter, got %q", f.Name())
	}
}
