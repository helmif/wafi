//go:build !windows

package memory

import (
	"fmt"
	"os"
)

// ttyNameFromFd resolves the tty path for fd via /dev/fd symlink.
// Works on Darwin and Linux without external packages.
func ttyNameFromFd(fd uintptr) (string, error) {
	link, err := os.Readlink(fmt.Sprintf("/dev/fd/%d", fd))
	if err != nil {
		return "", err
	}
	// Discard non-tty paths (pipes, sockets, regular files).
	if len(link) < 4 || link[:4] != "/dev" {
		return "", fmt.Errorf("not a tty: %s", link)
	}
	return link, nil
}
