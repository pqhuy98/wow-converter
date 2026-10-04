//go:build !windows

package reportshot

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
