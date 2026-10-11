package reportshot

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

//go:embed cancel-browser-downloads.ps1
var cancelBrowserDownloadsScript string

func hideWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

func cancelBrowserDownloads(tempDir string) error {
	// Use a fresh context: capture cancellation must not interrupt cleanup of service-owned jobs.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cancelBrowserDownloadsScript)
	cmd.Env = append(os.Environ(), "WOW_REPORT_SHOT_TEMP="+tempDir)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cancel browser BITS downloads: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if count := strings.TrimSpace(string(out)); count != "0" {
		log.Printf("report screenshot: cancelled %s browser BITS download(s)", count)
	}
	return nil
}
