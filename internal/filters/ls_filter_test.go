package filters

import (
	"testing"
)

func TestLsMatch(t *testing.T) {
	f := Ls{}
	cases := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"ls", []string{"-la"}, true},
		{"ls", []string{"-l"}, true},
		{"ls", []string{"-lh"}, true},
		{"ls", []string{"-al"}, true},
		{"ls", []string{"-lR"}, true},
		{"ls", []string{"-a"}, false},   // no -l
		{"ls", []string{}, false},        // no flags
		{"ls", []string{"-R"}, false},    // no -l
		{"ls", []string{"--long"}, false}, // long-opt, not short -l
		{"git", []string{"-l"}, false},   // wrong command
	}
	for _, c := range cases {
		got := f.Match(c.cmd, c.args)
		if got != c.want {
			t.Errorf("Match(%q, %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}

func TestLsApply(t *testing.T) {
	f := Ls{}
	ctx := ApplyContext{Cmd: "ls", Args: []string{"-la"}}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "basic -la",
			input: `total 48
drwxr-xr-x   5 alice  staff   160 Apr 20 12:00 .
drwxr-xr-x  15 alice  staff   480 Apr 19 11:00 ..
drwxr-xr-x   3 alice  staff    96 Apr 20 11:30 internal
-rw-r--r--   1 alice  staff   423 Apr 20 10:15 go.mod
-rw-r--r--   1 alice  staff  1532 Apr 20 10:15 README.md
`,
			want: `internal/
go.mod  423B
README.md  1.5K
`,
		},
		{
			name: "ls -lh (already human-readable sizes)",
			input: `total 48
drwxr-xr-x  3 alice  staff    96B Apr 20 11:30 .
drwxr-xr-x  5 alice  staff   160B Apr 19 11:00 ..
-rw-r--r--  1 alice  staff  1.2K Apr 20 10:15 main.go
-rw-r--r--  1 alice  staff  3.4M Apr 20 10:15 data.bin
`,
			want: `main.go  1.2K
data.bin  3.4M
`,
		},
		{
			name: "empty directory",
			input: `total 0
drwxr-xr-x  2 alice  staff   64 Apr 20 12:00 .
drwxr-xr-x  3 alice  staff   96 Apr 19 11:00 ..
`,
			want: "(empty)\n",
		},
		{
			name: "symlink",
			input: `total 8
lrwxrwxrwx  1 alice  staff  10 Apr 20 12:00 link -> target
-rw-r--r--  1 alice  staff 512 Apr 20 12:00 plain.txt
`,
			want: `link -> target  10B
plain.txt  512B
`,
		},
		{
			name: "old file with year instead of time",
			input: `total 8
-rw-r--r--  1 alice  staff  2048 Jan  5  2020 old.txt
-rw-r--r--  1 alice  staff   512 Apr 20 12:00 new.txt
`,
			want: `old.txt  2.0K
new.txt  512B
`,
		},
		{
			name: "no long-format lines — passthrough",
			input: `foo.go
bar.go
baz.go
`,
			want: `foo.go
bar.go
baz.go
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(f.Apply([]byte(c.input), ctx))
			if got != c.want {
				t.Errorf("Apply():\ngot:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

func TestLsRecursive(t *testing.T) {
	f := Ls{}
	ctx := ApplyContext{Cmd: "ls", Args: []string{"-lR"}}

	input := `.:
total 8
drwxr-xr-x  2 alice  staff   64 Apr 20 12:00 .
drwxr-xr-x  3 alice  staff   96 Apr 19 11:00 ..
drwxr-xr-x  2 alice  staff   64 Apr 20 12:00 sub
-rw-r--r--  1 alice  staff  100 Apr 20 12:00 root.go

./sub:
total 4
drwxr-xr-x  2 alice  staff  64 Apr 20 12:00 .
drwxr-xr-x  3 alice  staff  96 Apr 19 11:00 ..
-rw-r--r--  1 alice  staff 200 Apr 20 12:00 child.go
`
	want := `.:
sub/
root.go  100B

./sub:
child.go  200B
`
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("recursive Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}
