package filters

import (
	"os"
	"path/filepath"
	"testing"

	"wafi/internal/testutil"
)

func TestDockerBuild_Match(t *testing.T) {
	f := DockerBuild{}
	tests := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"docker", []string{"build", "."}, true},
		{"docker", []string{"build", "-t", "myapp:latest", "."}, true},
		{"docker", []string{"build", "--no-cache", "."}, true},
		{"docker", []string{"buildx", "build", "."}, true},
		{"docker", []string{"buildx", "build", "--platform", "linux/amd64", "."}, true},
		// should not match
		{"docker", []string{"run", "myapp"}, false},
		{"docker", []string{"push", "myapp"}, false},
		{"docker", []string{"buildx"}, false}, // buildx without subcommand
		{"docker", nil, false},
		{"podman", []string{"build", "."}, false},
	}
	for _, tt := range tests {
		got := f.Match(tt.cmd, tt.args)
		if got != tt.want {
			t.Errorf("Match(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestDockerBuild_Golden(t *testing.T) {
	tests := []string{"success", "cached", "error"}
	ctx := ApplyContext{Cmd: "docker", Args: []string{"build", "."}}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("testdata", "docker_build_"+name+".txt"))
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			got := DockerBuild{}.Apply(input, ctx)
			testutil.CheckGolden(t, "docker_build_"+name+".golden.txt", got)
		})
	}
}

func TestDockerBuild_EmptyInput(t *testing.T) {
	got := DockerBuild{}.Apply([]byte{}, ApplyContext{})
	if len(got) != 0 {
		t.Fatalf("empty input should return empty, got %q", got)
	}
}

func TestDockerBuild_NonBuildKit_PassThrough(t *testing.T) {
	// Old-style docker output has no matching drop patterns — should pass through.
	input := []byte("Sending build context to Docker daemon  2.048kB\nStep 1/2 : FROM alpine\n ---> a1b2c3d4e5f6\nSuccessfully built a1b2c3d4e5f6\n")
	got := DockerBuild{}.Apply(input, ApplyContext{Cmd: "docker", Args: []string{"build", "."}})
	if string(got) != string(input) {
		t.Fatalf("non-BuildKit output should pass through:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestDefault_RegistersDockerBuild(t *testing.T) {
	r := Default()
	f := r.Lookup("docker", []string{"build", "."})
	if f == nil {
		t.Fatal("Default() registry did not route `docker build`")
	}
	if f.Name() != "docker-build" {
		t.Fatalf("expected docker-build filter, got %q", f.Name())
	}
}
