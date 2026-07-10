package pickfolder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const pickTimeout = 5 * time.Minute

// ResolveInitialDirectory returns an existing directory, or its nearest existing parent.
func ResolveInitialDirectory(initial string) string {
	trimmed := strings.TrimSpace(initial)
	if trimmed == "" {
		return ""
	}

	candidate := filepath.Clean(trimmed)
	for i := 0; i < 32; i++ {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}
	return ""
}

// PickNativeFolder opens a native folder picker on the machine running the server.
// Returns an empty string when the user cancels.
func PickNativeFolder(title string, initialDirectory string) (string, error) {
	if title == "" {
		title = "Select folder"
	}
	initial := ResolveInitialDirectory(initialDirectory)

	ctx, cancel := context.WithTimeout(context.Background(), pickTimeout)
	defer cancel()

	switch runtime.GOOS {
	case "windows":
		return pickFolderWindows(ctx, title, initial)
	case "darwin":
		return pickFolderMac(ctx, title, initial)
	default:
		return pickFolderLinux(ctx, title, initial)
	}
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		return "", err
	}
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return "", nil
	}
	return selected, nil
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func pickFolderWindows(ctx context.Context, title, initialDirectory string) (string, error) {
	parts := []string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"[System.Windows.Forms.Application]::EnableVisualStyles()",
		"$dialog = New-Object System.Windows.Forms.FolderBrowserDialog",
		fmt.Sprintf("$dialog.Description = %s", psQuote(title)),
		"$dialog.UseDescriptionForTitle = $true",
		"$dialog.ShowNewFolderButton = $false",
	}
	if initialDirectory != "" {
		parts = append(parts, fmt.Sprintf("$dialog.SelectedPath = %s", psQuote(initialDirectory)))
	}
	parts = append(parts,
		"if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {",
		"  Write-Output $dialog.SelectedPath",
		"}",
	)
	script := strings.Join(parts, "; ")
	return run(ctx, "powershell", "-NoProfile", "-STA", "-Command", script)
}

func pickFolderMac(ctx context.Context, title, initialDirectory string) (string, error) {
	escapedTitle := strings.ReplaceAll(strings.ReplaceAll(title, `\`, `\\`), `"`, `\"`)
	defaultLocation := ""
	if initialDirectory != "" {
		escapedPath := strings.ReplaceAll(strings.ReplaceAll(initialDirectory, `\`, `\\`), `"`, `\"`)
		defaultLocation = fmt.Sprintf(` default location (POSIX file "%s")`, escapedPath)
	}
	script := fmt.Sprintf(`POSIX path of (choose folder with prompt "%s"%s)`, escapedTitle, defaultLocation)
	return run(ctx, "osascript", "-e", script)
}

func pickFolderLinux(ctx context.Context, title, initialDirectory string) (string, error) {
	zenityArgs := []string{"--file-selection", "--directory", "--title=" + title}
	if initialDirectory != "" {
		withSep := initialDirectory
		if !strings.HasSuffix(withSep, string(os.PathSeparator)) {
			withSep += string(os.PathSeparator)
		}
		zenityArgs = append(zenityArgs, "--filename="+withSep)
	}
	if selected, err := run(ctx, "zenity", zenityArgs...); err != nil {
		return "", err
	} else if selected != "" {
		return selected, nil
	}

	startDir := initialDirectory
	if startDir == "" {
		startDir = "."
	}
	return run(ctx, "kdialog", "--getexistingdirectory", startDir, "--title", title)
}
