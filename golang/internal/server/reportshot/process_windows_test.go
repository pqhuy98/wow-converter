package reportshot

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestBrowserTempCleanupWithBITS(t *testing.T) {
	if os.Getenv("REPORT_SHOT_BROWSER_TEST") != "1" {
		t.Skip("Set REPORT_SHOT_BROWSER_TEST=1 for the installed-browser/BITS cleanup check")
	}
	parent := t.TempDir()
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, parent)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	b, err := openBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	root := b.tempDir
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("BITS local source: %s range=%s", r.Method, r.Header.Get("Range"))
		http.ServeContent(w, r, "component.crx", time.Unix(1, 0), &slowBITSFile{Reader: bytes.NewReader(make([]byte, 2<<20)), ctx: r.Context(), release: release})
	}))
	defer server.Close()
	defer close(release)
	// Same-prefix sibling and mixed-destination jobs must never be mistaken for capture-owned jobs.
	sibling := root + "-other"
	for _, dir := range []string{filepath.Join(root, "msedgeedge_BITS_test"), sibling} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	testPS := func(script string, env ...string) string {
		t.Helper()
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; Import-Module BitsTransfer; "+script)
		cmd.Env = append(os.Environ(), env...)
		hideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("BITS test command: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	startJob := func(destinations []string, suspended bool) string {
		t.Helper()
		option := ""
		if suspended {
			option = " -Suspended"
		}
		id := testPS(`$dest=$env:SHOT_TEST_DEST.Split('|'); $sources=@($dest | ForEach-Object { $env:SHOT_TEST_SOURCE }); $job=Start-BitsTransfer -Source $sources -Destination $dest -DisplayName 'wow-converter cleanup regression' -Asynchronous -Priority Foreground -ProxyUsage NoProxy`+option+`; $job.JobId.ToString()`, "SHOT_TEST_DEST="+strings.Join(destinations, "|"), "SHOT_TEST_SOURCE="+server.URL+"/component.crx")
		t.Cleanup(func() {
			testPS(`Get-BitsTransfer | Where-Object { $_.JobId.ToString() -eq $env:SHOT_TEST_ID } | Remove-BitsTransfer -Confirm:$false`, "SHOT_TEST_ID="+id)
		})
		return id
	}
	owned := startJob([]string{filepath.Join(root, "msedgeedge_BITS_test", "component.crx")}, false)
	unrelated := startJob([]string{filepath.Join(sibling, "unrelated.crx")}, true)
	mixed := startJob([]string{filepath.Join(root, "mixed.crx"), filepath.Join(sibling, "mixed.crx")}, true)
	// BITS can hold its file while Connecting, before transfer progress becomes visible.
	testPS(`$deadline=(Get-Date).AddSeconds(30); while (-not (Get-ChildItem -LiteralPath $env:SHOT_TEST_DIR -Filter 'BIT*.tmp' -Force)) { if ((Get-Date) -gt $deadline) { throw 'BITS did not create its temp file' }; Start-Sleep -Milliseconds 100 }`, "SHOT_TEST_DIR="+filepath.Join(root, "msedgeedge_BITS_test"))
	temps, err := filepath.Glob(filepath.Join(root, "msedgeedge_BITS_test", "BIT*.tmp"))
	if err != nil || len(temps) != 1 {
		t.Fatalf("expected BITS temp file, got %v %v", temps, err)
	}
	name, err := windows.UTF16PtrFromString(temps[0])
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		windows.CloseHandle(handle)
		t.Fatal("BITS temp file was not locked")
	}
	if err != windows.ERROR_SHARING_VIOLATION {
		t.Fatalf("expected a service-owned sharing violation, got %v", err)
	}
	t.Logf("reproduced BITS service lock on %s", filepath.Base(temps[0]))
	b.close()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("capture temp root survived close: %v", err)
	}
	jobs := testPS(`@(Get-BitsTransfer | ForEach-Object { $_.JobId.ToString() }) -join ','`)
	if strings.Contains(jobs, owned) || !strings.Contains(jobs, unrelated) || !strings.Contains(jobs, mixed) {
		t.Errorf("wrong BITS jobs remained: owned=%s unrelated=%s mixed=%s jobs=%s", owned, unrelated, mixed, jobs)
	}
}

type slowBITSFile struct {
	*bytes.Reader
	ctx     context.Context
	release <-chan struct{}
}

func (file *slowBITSFile) Read(buffer []byte) (int, error) {
	if offset, _ := file.Seek(0, io.SeekCurrent); offset >= 32768 {
		select {
		case <-file.ctx.Done():
			return 0, file.ctx.Err()
		case <-file.release:
		}
	}
	return file.Reader.Read(buffer)
}
