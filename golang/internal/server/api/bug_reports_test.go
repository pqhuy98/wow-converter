package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/server/util"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func reportTestServer(t *testing.T) (*httptest.Server, *bugReportStore) {
	t.Helper()
	t.Setenv("BUG_REPORT_ADMIN_PASSWORD", "test-admin")
	store := &bugReportStore{dir: t.TempDir()}
	root := chi.NewRouter()
	root.Use(reportPeer)
	api := chi.NewRouter()
	registerBugReportRoutes(api, root, store)
	root.Mount("/api", api)
	server := httptest.NewServer(root)
	t.Cleanup(func() {
		server.Close()
		if store.db != nil {
			store.db.Close()
		}
	})
	return server, store
}

func testReportPayload() bugReportPayload {
	return bugReportPayload{ID: uuid.NewString(), WowheadURL: "https://www.wowhead.com/npc=36597/the-lich-king", Request: exportCharacterRequest{Character: character.Character{Base: character.WowheadRef("https://www.wowhead.com/npc=36597/the-lich-king"), InGameMovespeed: 270}, OutputFileName: "lich-king", Format: "mdx", FormatVersion: "1000"}, Sequence: character.SequenceSource{Index: 0, Name: "Stand 1", WowName: "Stand"}, Notes: "The sword texture is wrong", GitSHA: "abcdef", BuildKey: "test-build", Product: "wow", ExportedAt: time.Now().UnixMilli()}
}

func testReportPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1920, 800))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 200, G: 100, B: 50, A: 255}), image.Point{}, draw.Src)
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func reportUpload(t *testing.T, server *httptest.Server, payload bugReportPayload, images int, ip string) (int, []byte) {
	t.Helper()
	var data bytes.Buffer
	form := multipart.NewWriter(&data)
	if err := form.WriteField("report", string(mustJSON(payload))); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"wowhead", "converter"} {
		if i >= images {
			break
		}
		part, err := form.CreateFormFile(name, name+".png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(testReportPNG(t)); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("POST", server.URL+"/api/bug-reports", &data)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

func reportHTTP(t *testing.T, server *httptest.Server, method, path, password string, body io.Reader) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, server.URL+path, body)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if password != "" {
		req.Header.Set("Authorization", "Bearer "+password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, data
}

func TestBugReportsAccessUploadRetryAndDelete(t *testing.T) {
	server, store := reportTestServer(t)
	payload := testReportPayload()
	if code, _ := reportHTTP(t, server, "GET", "/api/bug-reports", "", nil); code != 401 {
		t.Fatalf("anonymous listing: %d", code)
	}
	if code, _ := reportUpload(t, server, payload, 1, "203.0.113.7"); code != 400 {
		t.Fatalf("missing image: %d", code)
	}
	t.Setenv("BUG_REPORT_SERVER_URL", "http://report.test")
	if code, body := reportUpload(t, server, payload, 2, "203.0.113.7"); code != 201 {
		t.Fatalf("upload: %d %s", code, body)
	} else if !bytes.Contains(body, []byte(`"url":"http://report.test/bug-reports/`+payload.ID)) {
		t.Fatalf("upload view url: %s", body)
	}
	if code, body := reportUpload(t, server, payload, 2, "203.0.113.7"); code != 200 {
		t.Fatalf("retry: %d %s", code, body)
	} else if !bytes.Contains(body, []byte(`"url":"http://report.test/bug-reports/`+payload.ID)) {
		t.Fatalf("retry view url: %s", body)
	}
	code, body := reportHTTP(t, server, "GET", "/api/bug-reports/"+payload.ID, "", nil)
	if code != 200 {
		t.Fatalf("anonymous detail: %d %s", code, body)
	}
	for _, secret := range []string{"\"ip\"", "submissions.sqlite", store.dir, "assetHashes", "outputDirectory", "email"} {
		if strings.Contains(string(body), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	if code, _ = reportHTTP(t, server, "POST", "/api/bug-reports/"+payload.ID, "", strings.NewReader(`{"status":"fixed"}`)); code != 401 {
		t.Fatalf("anonymous write: %d", code)
	}
	if code, _ = reportHTTP(t, server, "GET", "/api/bug-reports/"+payload.ID, "wrong", nil); code != 401 {
		t.Fatalf("invalid cached password: %d", code)
	}
	if code, body = reportHTTP(t, server, "POST", "/api/bug-reports/"+payload.ID, "test-admin", strings.NewReader(`{"status":"fixed","notes":"Updated notes"}`)); code != 200 || !strings.Contains(string(body), "Updated notes") {
		t.Fatalf("admin edit: %d %s", code, body)
	}
	if code, body = reportHTTP(t, server, "GET", "/api/bug-reports?search=Updated&status=fixed&page=1", "test-admin", nil); code != 200 || !strings.Contains(string(body), `"total":1`) {
		t.Fatalf("filtered list: %d %s", code, body)
	}
	if code, _ = reportHTTP(t, server, "GET", "/bug-reports/images/"+payload.ID+"_1.png", "", nil); code != 200 {
		t.Fatalf("image: %d", code)
	}
	if code, _ = reportHTTP(t, server, "POST", "/api/bug-reports/"+payload.ID, "test-admin", strings.NewReader(`{"delete":true}`)); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ = reportHTTP(t, server, "GET", "/api/bug-reports/"+payload.ID, "", nil); code != 404 {
		t.Fatalf("deleted detail: %d", code)
	}
	if code, _ = reportHTTP(t, server, "GET", "/bug-reports/images/"+payload.ID+"_1.png", "", nil); code != 404 {
		t.Fatalf("deleted image: %d", code)
	}
	if code, _ = reportHTTP(t, server, "GET", "/api/bug-reports/quota", "", nil); code != 200 {
		t.Fatalf("quota: %d", code)
	}
	if code, body = reportHTTP(t, server, "GET", "/api/bug-reports/quota", "", nil); !strings.Contains(string(body), `"remaining":9`) {
		t.Fatalf("delete refunded quota: %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(store.dir, payload.ID+"_1.png")); !os.IsNotExist(err) {
		t.Fatal("deleted screenshot still exists")
	}
}

func TestBugReportsQuotaPersistsAndRejectsConcurrentEleventh(t *testing.T) {
	server, store := reportTestServer(t)
	for i := 0; i < 9; i++ {
		if code, body := reportUpload(t, server, testReportPayload(), 2, "203.0.113.7"); code != 201 {
			t.Fatalf("upload %d: %d %s", i, code, body)
		}
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := reportUpload(t, server, testReportPayload(), 2, "203.0.113.7")
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	accepted, rejected := 0, 0
	for code := range codes {
		if code == 201 {
			accepted++
		}
		if code == 429 {
			rejected++
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("concurrent quota: accepted %d rejected %d", accepted, rejected)
	}
	reopened := &bugReportStore{dir: store.dir}
	if err := reopened.open(); err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	start, _ := reportDay()
	var count int
	if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE created_at>=?`, start).Scan(&count); err != nil || count != 10 {
		t.Fatalf("durable quota %d %v", count, err)
	}
	if code, _ := reportUpload(t, server, testReportPayload(), 2, "203.0.113.8"); code != 201 {
		t.Fatalf("different IP: %d", code)
	}
}

func TestBugReportsRejectPathsAndInvalidImages(t *testing.T) {
	payload := testReportPayload()
	payload.Request.Character.AttachItems = map[string]character.AttachItem{"1": {Path: character.LocalRef(`C:\Users\private\secret.mdx`)}}
	if err := validateReportPayload(payload); err == nil {
		t.Fatal("accepted filesystem path")
	}
	payload = testReportPayload()
	payload.WowheadURL = "https://wowhead.com.attacker.example/npc=1"
	if err := validateReportPayload(payload); err == nil {
		t.Fatal("accepted fake Wowhead URL")
	}
	if err := validateReportImage([]byte("not a PNG")); err == nil {
		t.Fatal("accepted invalid image")
	}
	if err := validateReportImage(make([]byte, reportImageLimit+1)); err == nil {
		t.Fatal("accepted oversized image")
	}
	var body map[string]any
	_ = json.Unmarshal(mustJSON(testReportPayload()), &body)
	if _, exists := body["email"]; exists {
		t.Fatal("email collected")
	}
}

func TestReportPeerTrustsOnlyLocalProxyAndLastForwardedIP(t *testing.T) {
	get := func(remote, forwarded string) string {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remote
		req.Header.Set("X-Forwarded-For", forwarded)
		var ip string
		reportPeer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ip = reportClientIP(r) })).ServeHTTP(httptest.NewRecorder(), req)
		return ip
	}
	if get("198.51.100.1:80", "203.0.113.7") != get("198.51.100.1:80", "") {
		t.Fatal("untrusted peer spoofed IP")
	}
	if get("127.0.0.1:80", "spoofed, 203.0.113.7") != get("203.0.113.7:80", "") {
		t.Fatal("wrong nginx forwarded IP")
	}
}

func TestReportDetectsOverwrittenExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.mdx")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := hashExportAsset(dir, "model.mdx")
	if err != nil {
		t.Fatal(err)
	}
	metadata := &exportReportMetadata{AssetHashes: map[string]string{"model.mdx": hash}}
	if err = checkExportAssets(metadata, dir); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = checkExportAssets(metadata, dir); err == nil {
		t.Fatal("changed export accepted")
	}
}

// Opt-in local fixture for browser QA. It never contacts the production report receiver.
func TestBugReportsUIFixture(t *testing.T) {
	if os.Getenv("REPORT_UI_FIXTURE") != "1" {
		t.Skip("Set REPORT_UI_FIXTURE=1 for a temporary report UI server")
	}
	t.Setenv("BUG_REPORT_ADMIN_PASSWORD", "test-admin")
	store := &bugReportStore{dir: t.TempDir()}
	root := chi.NewRouter()
	root.Use(reportPeer)
	api := chi.NewRouter()
	registerBugReportRoutes(api, root, store)
	api.Get("/get-config", func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, 200, map[string]any{"isSharedHosting": false, "exportAssetDir": "", "isClassic": false, "buildKey": "test"})
	})
	api.Get("/wow-config/status", func(w http.ResponseWriter, _ *http.Request) { sendJSON(w, 200, map[string]any{"cascLoaded": true}) })
	root.Mount("/api", api)
	if !registerWebUI(root, &Deps{Config: Config{}}, workspace.ResolveRepoPath("webui/out")) {
		t.Fatal("Build the web UI first")
	}
	server := httptest.NewServer(root)
	defer server.Close()
	t.Setenv("BUG_REPORT_SERVER_URL", server.URL)
	// Reuse a completed real export when available, so prepare/preview/submit can be checked end to end.
	var recent []*util.Job[exportCharacterRequest, exportCharacterResponse]
	if data, err := os.ReadFile(workspace.ResolveRepoPath(recentExportsRel)); err == nil {
		_ = json.Unmarshal(data, &recent)
	}
	for _, job := range recent {
		if job.Result == nil || job.Result.ReportMetadata == nil || job.Request.OutputFileName != "report-verification-lich-king" {
			continue
		}
		parsed, _ := url.Parse(server.URL)
		port, _ := strconv.Atoi(parsed.Port())
		deps := &Deps{Config: Config{Port: port, OutputDir: workspace.ResolveRepoPath(outputDirRel), OutputDirBrowse: workspace.ResolveRepoPath(outputDirBrowseRel)}}
		exports := util.NewJobQueue(util.QueueConfig[exportCharacterRequest, exportCharacterResponse]{Concurrency: 1, JobTTL: time.Hour, JobTimeout: time.Minute}, func(_ *util.Job[exportCharacterRequest, exportCharacterResponse]) (exportCharacterResponse, error) {
			return *job.Result, nil
		})
		exports.RecentCompletedJobs = []*util.Job[exportCharacterRequest, exportCharacterResponse]{job}
		registerExportStatic(api, deps)
		registerReportCapture(api, deps, exports)
		api.Get("/export/character/demos", func(w http.ResponseWriter, _ *http.Request) { sendJSON(w, 200, []any{}) })
		api.Post("/export/character", func(w http.ResponseWriter, _ *http.Request) {
			sendJSON(w, 200, util.JobStatusView[exportCharacterResponse]{ID: job.ID, Status: util.JobPending})
		})
		api.Get("/export/character/status/{id}", func(w http.ResponseWriter, _ *http.Request) {
			sendJSON(w, 200, util.JobStatusView[exportCharacterResponse]{ID: job.ID, Status: util.JobDone, Result: job.Result})
		})
		break
	}
	if err := store.open(); err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	payload := testReportPayload()
	paths := [2]string{payload.ID + "_1.png", payload.ID + "_2.png"}
	for i, name := range []string{"wowhead.png", "converter.png"} {
		data, err := os.ReadFile(workspace.ResolveRepoPath("docs/screenshots/bug-reports/" + name))
		if err != nil {
			data = testReportPNG(t)
		}
		if err = os.WriteFile(filepath.Join(store.dir, paths[i]), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UnixMilli()
	if _, err := store.db.Exec(`INSERT INTO submissions(id,payload,images,ip,created_at,updated_at) VALUES(?,?,?,?,?,?)`, payload.ID, string(mustJSON(payload)), string(mustJSON(paths)), "test", now, now); err != nil {
		t.Fatal(err)
	}
	t.Logf("UI fixture: %s/bug-reports/%s (admin password test-admin)", server.URL, payload.ID)
	// Longer timeout gives the browser enough time for admin/read-only comparisons.
	time.Sleep(10 * time.Minute)
}

func TestReportProxyReturnsFriendlyJSONForUnavailableReceiver(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer receiver.Close()
	t.Setenv("BUG_REPORT_SERVER_URL", receiver.URL)
	response := httptest.NewRecorder()
	proxyReportRequest(response, httptest.NewRequest("GET", "/quota", nil), "GET", "/api/bug-reports/quota", nil, "")
	if response.Code != 502 || !json.Valid(response.Body.Bytes()) || !strings.Contains(response.Body.String(), "report server is unavailable") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestBugReportServerURLNormalizesConfiguredBase(t *testing.T) {
	t.Setenv("BUG_REPORT_SERVER_URL", " http://127.0.0.1:3001/ ")
	if got := bugReportServerURL(); got != "http://127.0.0.1:3001" {
		t.Fatalf("base URL = %q", got)
	}
}

func TestBugReportServerURLDefaultsByEnvironment(t *testing.T) {
	t.Setenv("BUG_REPORT_SERVER_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("NODE_ENV", "development")
	if got := bugReportServerURL(); got != "http://127.0.0.1:3001" {
		t.Fatalf("dev default = %q", got)
	}
	if got := bugReportViewURL("id"); got != "http://127.0.0.1:3001/bug-reports/id" {
		t.Fatalf("dev view = %q", got)
	}
	t.Setenv("PORT", "4002")
	if got := bugReportServerURL(); got != "http://127.0.0.1:4002" {
		t.Fatalf("dev port = %q", got)
	}
	t.Setenv("NODE_ENV", "production")
	if got := bugReportServerURL(); got != productionReportServerURL {
		t.Fatalf("prod default = %q", got)
	}
}

func TestReportQuotaProbesConfiguredDestination(t *testing.T) {
	seen := make(chan string, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.URL.Path
		sendJSON(w, 200, map[string]any{"remaining": 7, "resetAt": 1})
	}))
	defer receiver.Close()
	t.Setenv("BUG_REPORT_SERVER_URL", receiver.URL)
	response := httptest.NewRecorder()
	proxyReportRequest(response, httptest.NewRequest("GET", "/quota", nil), "GET", "/api/bug-reports/quota", nil, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"remaining":7`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
	if got := <-seen; got != "/api/bug-reports/quota" {
		t.Fatalf("quota path = %q", got)
	}
}

func TestReportForwardingToConfiguredLocalReceiver(t *testing.T) {
	receiver, _ := reportTestServer(t)
	t.Setenv("BUG_REPORT_SERVER_URL", " "+receiver.URL+"/ ")
	payload := testReportPayload()
	var data bytes.Buffer
	form := multipart.NewWriter(&data)
	if err := form.WriteField("report", string(mustJSON(payload))); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wowhead", "converter"} {
		part, err := form.CreateFormFile(name, name+".png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(testReportPNG(t)); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	response, err := remoteReportRequest(context.Background(), "POST", "/api/bug-reports", &data, form.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 201 {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("forwarded upload: %d %s", response.StatusCode, body)
	} else {
		body, _ := io.ReadAll(response.Body)
		if !bytes.Contains(body, []byte(`"url":"`+receiver.URL+"/bug-reports/"+payload.ID)) {
			t.Fatalf("forwarded view url: %s", body)
		}
	}
	if code, body := reportHTTP(t, receiver, "GET", "/api/bug-reports/"+payload.ID, "", nil); code != 200 || !bytes.Contains(body, []byte(payload.Notes)) {
		t.Fatalf("forwarded report not readable: %d %s", code, body)
	}
}
