//go:build !windows

package reportshot

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}

func cancelBrowserDownloads(tempDir string) error { return nil }
