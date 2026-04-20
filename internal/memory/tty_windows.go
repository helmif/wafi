//go:build windows

package memory

import "fmt"

func ttyNameFromFd(fd uintptr) (string, error) {
	return "", fmt.Errorf("tty detection not supported on windows")
}
