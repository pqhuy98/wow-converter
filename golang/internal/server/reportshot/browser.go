// Package reportshot captures comparison sheets with the user's installed browser.
package reportshot

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type browser struct {
	cmd           *exec.Cmd
	conn          *websocket.Conn
	profile       string
	tempDir       string
	address       string
	assetRequests map[string]bool
	assetActivity time.Time
	next          int
}

func openBrowser(ctx context.Context) (*browser, error) {
	executable, err := findBrowser()
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "wow-report-shot-")
	if err != nil {
		return nil, err
	}
	b := &browser{tempDir: tempDir, profile: filepath.Join(tempDir, "profile")}
	b.cmd = exec.CommandContext(ctx, executable, "--headless=new", "--remote-debugging-port=0", "--user-data-dir="+b.profile,
		"--no-first-run", "--no-default-browser-check", "--disable-extensions", "--hide-scrollbars", "--window-size=1440,900",
		"--disable-background-networking", "--disable-component-update", "--disable-sync",
		"--disable-background-timer-throttling", "--disable-renderer-backgrounding", "--disable-backgrounding-occluded-windows",
		"--force-device-scale-factor=1", "--enable-webgl", "--ignore-gpu-blocklist", "about:blank")
	// A profile alone does not contain Edge/Chromium's component downloads and scratch files.
	b.cmd.Env = append(os.Environ(), "TEMP="+tempDir, "TMP="+tempDir, "TMPDIR="+tempDir)
	// CommandContext normally kills only the parent, leaving Windows children and locked files.
	b.cmd.Cancel = b.kill
	b.cmd.WaitDelay = 5 * time.Second
	hideWindow(b.cmd)
	stdout, err := b.cmd.StdoutPipe()
	if err != nil {
		b.close()
		return nil, err
	}
	stderr, err := b.cmd.StderrPipe()
	if err != nil {
		b.close()
		return nil, err
	}
	if err = b.cmd.Start(); err != nil {
		b.close()
		return nil, errors.New("Could not start your browser. Close other screenshot attempts and try again.")
	}
	address, err := waitDevTools(ctx, b.profile, stdout, stderr)
	if err != nil {
		b.close()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, errors.New("Your browser did not start screenshot capture. Please try again.")
	}
	b.address = address
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+address+"/json/list", nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		b.close()
		return nil, err
	}
	var pages []struct {
		Type string `json:"type"`
		URL  string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&pages)
	resp.Body.Close()
	if err != nil {
		b.close()
		return nil, err
	}
	for _, page := range pages {
		if page.Type == "page" {
			b.conn, _, err = websocket.DefaultDialer.DialContext(ctx, page.URL, nil)
			break
		}
	}
	if err != nil || b.conn == nil {
		b.close()
		return nil, errors.New("Could not connect to the screenshot browser.")
	}
	b.conn.SetReadLimit(32 << 20)
	for _, method := range []string{"Page.enable", "Network.enable"} {
		if _, err = b.call(ctx, method, nil); err != nil {
			b.close()
			return nil, err
		}
	}
	return b, nil
}

func waitDevTools(ctx context.Context, profile string, streams ...io.Reader) (string, error) {
	ready := make(chan string, 1)
	send := func(address string) {
		if address == "" {
			return
		}
		select {
		case ready <- address:
		default:
		}
	}
	for _, stream := range streams {
		if stream == nil {
			continue
		}
		go func(r io.Reader) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				send(devToolsAddress(scanner.Text()))
			}
		}(stream)
	}
	deadline := time.Now().Add(20 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case address := <-ready:
			return address, nil
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
			if address := readDevToolsAddress(profile); address != "" {
				return address, nil
			}
			if !time.Now().Before(deadline) {
				return "", errors.New("Your browser did not start screenshot capture. Please try again.")
			}
		}
	}
}

func devToolsAddress(line string) string {
	at := strings.Index(line, "DevTools listening on ")
	if at < 0 {
		return ""
	}
	socket := strings.TrimSpace(line[at+len("DevTools listening on "):])
	return strings.Split(strings.TrimPrefix(socket, "ws://"), "/")[0]
}

func readDevToolsAddress(profile string) string {
	data, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
	if err != nil {
		return ""
	}
	port := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	if _, err = strconv.Atoi(port); err != nil || port == "0" {
		return ""
	}
	return "127.0.0.1:" + port
}

// Each tab has its own CDP connection, so capture can run concurrently in one browser.
func (b *browser) newTab(ctx context.Context) (*browser, error) {
	req, _ := http.NewRequestWithContext(ctx, "PUT", "http://"+b.address+"/json/new?about:blank", nil)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var target struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	if err = json.NewDecoder(response.Body).Decode(&target); err != nil {
		return nil, err
	}
	tab := &browser{}
	tab.conn, _, err = websocket.DefaultDialer.DialContext(ctx, target.URL, nil)
	if err != nil {
		return nil, err
	}
	tab.conn.SetReadLimit(32 << 20)
	for _, method := range []string{"Page.enable", "Network.enable"} {
		if _, err = tab.call(ctx, method, nil); err != nil {
			tab.close()
			return nil, err
		}
	}
	return tab, nil
}

func (b *browser) setCacheDisabled(ctx context.Context, disabled bool) error {
	_, err := b.call(ctx, "Network.setCacheDisabled", map[string]bool{"cacheDisabled": disabled})
	return err
}

func (b *browser) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	b.next++
	deadline := time.Now().Add(25 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	b.conn.SetWriteDeadline(deadline)
	b.conn.SetReadDeadline(deadline)
	if err := b.conn.WriteJSON(map[string]any{"id": b.next, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		var response struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := b.conn.ReadJSON(&response); err != nil {
			return nil, err
		}
		b.trackAssetRequest(response.Method, response.Params)
		if response.ID != b.next {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("capture %s: %s", method, response.Error.Message)
		}
		return response.Result, nil
	}
}

// Viewer readiness precedes its asynchronous body, equipment and texture loads.
// Ignore page ads and analytics, which may keep loading indefinitely.
func (b *browser) trackAssetRequest(method string, raw json.RawMessage) {
	if method != "Network.requestWillBeSent" && method != "Network.loadingFinished" && method != "Network.loadingFailed" {
		return
	}
	var event struct {
		RequestID string `json:"requestId"`
		Request   struct {
			URL string `json:"url"`
		} `json:"request"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	if method == "Network.requestWillBeSent" {
		if !strings.Contains(event.Request.URL, "/modelviewer/") {
			return
		}
		if b.assetRequests == nil {
			b.assetRequests = map[string]bool{}
		}
		b.assetRequests[event.RequestID] = true
	} else {
		if !b.assetRequests[event.RequestID] {
			return
		}
		delete(b.assetRequests, event.RequestID)
	}
	b.assetActivity = time.Now()
}

func (b *browser) assetsReady() bool {
	return len(b.assetRequests) == 0 && !b.assetActivity.IsZero() && time.Since(b.assetActivity) >= 750*time.Millisecond
}

func (b *browser) evaluate(ctx context.Context, expression string, dst any) error {
	raw, err := b.call(ctx, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true})
	if err != nil {
		return err
	}
	var value struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err = json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if len(value.Exception) != 0 {
		log.Printf("report screenshot: Runtime.evaluate failed: expression=%s exception=%s", expression[:min(len(expression), 160)], value.Exception)
		return errors.New("The model viewer could not render the screenshot. Please retake it.")
	}
	if dst == nil {
		return nil
	}
	return json.Unmarshal(value.Result.Value, dst)
}

func (b *browser) close() {
	if b.conn != nil && b.cmd != nil && b.cmd.Process != nil {
		// Give the browser a chance to stop its children and release profile handles.
		_ = b.conn.SetWriteDeadline(time.Now().Add(time.Second))
		b.next++
		_ = b.conn.WriteJSON(map[string]any{"id": b.next, "method": "Browser.close"})
	}
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
	if b.cmd != nil && b.cmd.Process != nil {
		done := make(chan struct{})
		go func() {
			_ = b.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = b.kill()
			<-done
		}
	}
	b.cmd = nil
	if b.tempDir != "" {
		// Windows can retain handles briefly after process exit. Remove only this launch's root.
		var err error
		for attempt := 0; attempt < 20; attempt++ {
			if err = os.RemoveAll(b.tempDir); err == nil {
				b.tempDir = ""
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			log.Printf("report screenshot: could not remove browser temp directory %s: %v", b.tempDir, err)
		}
	}
}

func (b *browser) kill() error {
	if runtime.GOOS == "windows" {
		kill := exec.Command("taskkill", "/pid", strconv.Itoa(b.cmd.Process.Pid), "/T", "/F")
		hideWindow(kill)
		if err := kill.Run(); err == nil {
			return nil
		}
	}
	return b.cmd.Process.Kill()
}

func findBrowser() (string, error) {
	if runtime.GOOS == "windows" {
		for _, hive := range []string{"HKCU", "HKLM"} {
			for _, exe := range []string{"msedge.exe", "chrome.exe"} {
				cmd := exec.Command("reg", "query", hive+`\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exe, "/ve")
				hideWindow(cmd)
				out, _ := cmd.Output()
				for _, line := range strings.Split(string(out), "\n") {
					if _, path, ok := strings.Cut(line, "REG_SZ"); ok {
						path = strings.Trim(strings.TrimSpace(path), `"`)
						if info, err := os.Stat(path); err == nil && !info.IsDir() {
							return path, nil
						}
					}
				}
			}
		}
	}
	var paths []string
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if root != "" {
			paths = append(paths, filepath.Join(root, "Microsoft/Edge/Application/msedge.exe"), filepath.Join(root, "Google/Chrome/Application/chrome.exe"))
		}
	}
	paths = append(paths, "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", "/Applications/Chromium.app/Contents/MacOS/Chromium")
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("Screenshots need Chrome, Edge, or Chromium. Please install one of these browsers, then try again.")
}
