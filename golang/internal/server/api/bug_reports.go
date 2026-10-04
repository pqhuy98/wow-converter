package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	_ "modernc.org/sqlite"
)

const reportImageLimit = 8 << 20

type bugReportStore struct {
	dir  string
	once sync.Once
	db   *sql.DB
	err  error
	mu   sync.Mutex
}

type bugReportView struct {
	bugReportPayload
	Status    string    `json:"status"`
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
	Images    [2]string `json:"images"`
}

type reportPeerKey struct{}

// Preserve the socket peer before generic RealIP middleware rewrites RemoteAddr.
func reportPeer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip != nil && ip.IsLoopback() {
			// nginx appends the real client to proxy_add_x_forwarded_for. Earlier entries are untrusted.
			parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
			if forwarded := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); forwarded != nil {
				ip = forwarded
			}
		}
		value := "unknown"
		if ip != nil {
			value = ip.String()
		}
		hash := sha256.Sum256([]byte(value))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), reportPeerKey{}, hex.EncodeToString(hash[:]))))
	})
}

func reportClientIP(r *http.Request) string {
	value, _ := r.Context().Value(reportPeerKey{}).(string)
	return value
}

func reportStoreDir(cfg Config) string {
	if cfg.IsSharedHosting && !cfg.IsDev {
		return "/var/lib/wow-converter/.bug-reports"
	}
	return workspace.ResolveRepoPath(".bug-reports")
}

func (s *bugReportStore) open() error {
	s.once.Do(func() {
		if s.err = os.MkdirAll(s.dir, 0o700); s.err != nil {
			return
		}
		s.db, s.err = sql.Open("sqlite", filepath.Join(s.dir, "submissions.sqlite"))
		if s.err != nil {
			return
		}
		s.db.SetMaxOpenConns(1)
		_, s.err = s.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
		CREATE TABLE IF NOT EXISTS submissions (
		id TEXT PRIMARY KEY, payload TEXT NOT NULL, images TEXT NOT NULL, ip TEXT NOT NULL,
		created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','fixed','declined')),
		deleted INTEGER NOT NULL DEFAULT 0);
		CREATE INDEX IF NOT EXISTS submissions_ip_time ON submissions(ip,created_at);
		CREATE INDEX IF NOT EXISTS submissions_status_time ON submissions(deleted,status,created_at);`)
	})
	return s.err
}

func reportDay() (int64, int64) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return start.UnixMilli(), start.Add(24 * time.Hour).UnixMilli()
}

func validReportID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.Version() == 4 && u.String() == id
}

func reportAdmin(r *http.Request) bool {
	password := os.Getenv("BUG_REPORT_ADMIN_PASSWORD")
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return password != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(password)) == 1
}

func requireReportAdmin(w http.ResponseWriter, r *http.Request) bool {
	if reportAdmin(r) {
		return true
	}
	sendError(w, http.StatusUnauthorized, "Incorrect admin password.")
	return false
}

func (s *bugReportStore) view(id string) (bugReportView, error) {
	var view bugReportView
	var payload string
	err := s.db.QueryRow(`SELECT payload,status,created_at,updated_at FROM submissions WHERE id=? AND deleted=0`, id).Scan(&payload, &view.Status, &view.CreatedAt, &view.UpdatedAt)
	if err != nil {
		return view, err
	}
	if err = json.Unmarshal([]byte(payload), &view.bugReportPayload); err != nil {
		return view, err
	}
	view.Images = [2]string{"/bug-reports/images/" + id + "_1.png", "/bug-reports/images/" + id + "_2.png"}
	return view, nil
}

func registerBugReports(r Router, root chiRouter, d *Deps) {
	if !d.Config.IsSharedHosting && !d.Config.IsDev {
		return
	}
	store := &bugReportStore{dir: reportStoreDir(d.Config)}
	registerBugReportRoutes(r, root, store)
}

func registerBugReportRoutes(r Router, root chiRouter, store *bugReportStore) {
	ready := func(w http.ResponseWriter) bool {
		setNoStore(w)
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
		if err := store.open(); err != nil {
			log.Printf("bug reports storage: %v", err)
			sendError(w, 503, "Bug reports are temporarily unavailable. Please try again later.")
			return false
		}
		return true
	}
	r.Get("/bug-reports/quota", func(w http.ResponseWriter, req *http.Request) {
		if !ready(w) {
			return
		}
		start, reset := reportDay()
		var count int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE ip=? AND created_at>=?`, reportClientIP(req), start).Scan(&count); err != nil {
			sendInternalError(w, err)
			return
		}
		sendJSON(w, 200, map[string]any{"remaining": max(0, 10-count), "resetAt": reset})
	})
	r.Post("/bug-reports", func(w http.ResponseWriter, req *http.Request) {
		if !ready(w) {
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, 2*reportImageLimit+(256<<10))
		if err := req.ParseMultipartForm(256 << 10); err != nil {
			if req.MultipartForm != nil {
				_ = req.MultipartForm.RemoveAll()
			}
			sendError(w, 413, "The report is too large. Retake screenshots and try again.")
			return
		}
		defer req.MultipartForm.RemoveAll()
		if len(req.FormValue("report")) > 128<<10 {
			sendError(w, 413, "The report settings are too large.")
			return
		}
		var payload bugReportPayload
		decoder := json.NewDecoder(strings.NewReader(req.FormValue("report")))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil || !validReportID(payload.ID) {
			sendError(w, 400, "Invalid report submission.")
			return
		}
		if err := validateReportPayload(payload); err != nil {
			sendError(w, 400, err.Error())
			return
		}
		var images [2][]byte
		for i, name := range []string{"wowhead", "converter"} {
			files := req.MultipartForm.File[name]
			if len(files) != 1 || files[0].Size > reportImageLimit {
				sendError(w, 400, "Two screenshots, each no larger than 8 MB, are required.")
				return
			}
			file, err := files[0].Open()
			if err != nil {
				sendInternalError(w, err)
				return
			}
			images[i], err = io.ReadAll(io.LimitReader(file, reportImageLimit+1))
			file.Close()
			if err != nil {
				sendInternalError(w, err)
				return
			}
			if err = validateReportImage(images[i]); err != nil {
				sendError(w, 400, err.Error())
				return
			}
		}
		if len(req.MultipartForm.File) != 2 {
			sendError(w, 400, "Only the two screenshots can be uploaded.")
			return
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		var owner string
		err := store.db.QueryRow(`SELECT ip FROM submissions WHERE id=?`, payload.ID).Scan(&owner)
		if err == nil {
			if owner != reportClientIP(req) {
				sendError(w, 409, "This report identifier has already been used.")
				return
			}
			if view, e := store.view(payload.ID); e == nil {
				sendJSON(w, 200, map[string]any{"id": view.ID, "url": bugReportViewURL(view.ID)})
				return
			}
			sendError(w, 410, "This report was deleted. Prepare a new report to submit again.")
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			sendInternalError(w, err)
			return
		}
		tx, err := store.db.Begin()
		if err != nil {
			sendInternalError(w, err)
			return
		}
		defer tx.Rollback()
		start, reset := reportDay()
		var count int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM submissions WHERE ip=? AND created_at>=?`, reportClientIP(req), start).Scan(&count); err != nil {
			sendInternalError(w, err)
			return
		}
		if count >= 10 {
			w.Header().Set("Retry-After", strconv.FormatInt(max(1, (reset-time.Now().UnixMilli())/1000), 10))
			sendError(w, 429, "You have used all 10 reports for today. Please try again after midnight UTC.")
			return
		}
		paths := [2]string{payload.ID + "_1.png", payload.ID + "_2.png"}
		committed := false
		defer func() {
			if !committed {
				for _, path := range paths {
					_ = os.Remove(filepath.Join(store.dir, path))
				}
			}
		}()
		for i, path := range paths {
			if err = os.WriteFile(filepath.Join(store.dir, path), images[i], 0o600); err != nil {
				sendInternalError(w, err)
				return
			}
		}
		now := time.Now().UnixMilli()
		payload.Notes = strings.TrimSpace(payload.Notes)
		_, err = tx.Exec(`INSERT INTO submissions(id,payload,images,ip,created_at,updated_at) VALUES(?,?,?,?,?,?)`, payload.ID, string(mustJSON(payload)), string(mustJSON(paths)), reportClientIP(req), now, now)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		if err = tx.Commit(); err != nil {
			sendInternalError(w, err)
			return
		}
		committed = true
		sendJSON(w, 201, map[string]any{"id": payload.ID, "url": bugReportViewURL(payload.ID), "remaining": 9 - count})
	})
	r.Get("/bug-reports", func(w http.ResponseWriter, req *http.Request) {
		if !requireReportAdmin(w, req) || !ready(w) {
			return
		}
		query := req.URL.Query()
		page, _ := strconv.Atoi(query.Get("page"))
		page = max(1, min(page, 1000000))
		search := strings.TrimSpace(query.Get("search"))
		if len(search) > 200 {
			search = search[:200]
		}
		status := query.Get("status")
		if status != "" && status != "open" && status != "fixed" && status != "declined" {
			sendError(w, 400, "Invalid status filter.")
			return
		}
		// Search the user-facing source, notes and meaningful export settings; SQL parameters preserve literal input.
		where := `deleted=0 AND (?='' OR status=?) AND (?='' OR instr(lower(payload),lower(?))>0)`
		args := []any{status, status, search, search}
		var total int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE "+where, args...).Scan(&total); err != nil {
			sendInternalError(w, err)
			return
		}
		rows, err := store.db.Query("SELECT id FROM submissions WHERE "+where+" ORDER BY created_at DESC,id LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			ids = append(ids, id)
		}
		rowErr := rows.Err()
		rows.Close()
		if err != nil || rowErr != nil {
			sendError(w, 500, "Could not load reports.")
			return
		}
		items := make([]bugReportView, 0, len(ids))
		for _, id := range ids {
			view, err := store.view(id)
			if err == nil {
				items = append(items, view)
			}
		}
		sendJSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "pageSize": 12})
	})
	r.Get("/bug-reports/{id}", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "" && !requireReportAdmin(w, req) {
			return
		}
		if !ready(w) {
			return
		}
		id := chi.URLParam(req, "id")
		if !validReportID(id) {
			sendError(w, 404, "Report not found.")
			return
		}
		view, err := store.view(id)
		if errors.Is(err, sql.ErrNoRows) {
			sendError(w, 404, "Report not found.")
			return
		}
		if err != nil {
			sendInternalError(w, err)
			return
		}
		// The DTO deliberately excludes IP hashes and storage paths even for authenticated readers.
		sendJSON(w, 200, view)
	})
	r.Post("/bug-reports/{id}", func(w http.ResponseWriter, req *http.Request) {
		if !requireReportAdmin(w, req) || !ready(w) {
			return
		}
		id := chi.URLParam(req, "id")
		if !validReportID(id) {
			sendError(w, 404, "Report not found.")
			return
		}
		var edit struct {
			Status *string `json:"status"`
			Notes  *string `json:"notes"`
			Delete bool    `json:"delete"`
		}
		if err := readJSONBody(req, &edit); err != nil {
			sendError(w, 400, "Invalid update.")
			return
		}
		if edit.Status != nil && *edit.Status != "open" && *edit.Status != "fixed" && *edit.Status != "declined" {
			sendError(w, 400, "Invalid report status.")
			return
		}
		if edit.Notes != nil && (strings.TrimSpace(*edit.Notes) == "" || len(*edit.Notes) > 8000) {
			sendError(w, 400, "Notes must contain 1–8,000 characters.")
			return
		}
		store.mu.Lock()
		defer store.mu.Unlock()
		view, err := store.view(id)
		if err != nil {
			sendError(w, 404, "Report not found.")
			return
		}
		if edit.Delete {
			// ponytail: a minimal tombstone keeps the day's quota and retry identity after deletion.
			_, err = store.db.Exec(`UPDATE submissions SET deleted=1,payload='{}',images='[]',updated_at=? WHERE id=?`, time.Now().UnixMilli(), id)
			if err != nil {
				sendInternalError(w, err)
				return
			}
			for i := 1; i <= 2; i++ {
				_ = os.Remove(filepath.Join(store.dir, id+"_"+strconv.Itoa(i)+".png"))
			}
			sendJSON(w, 200, map[string]bool{"deleted": true})
			return
		}
		if edit.Status != nil {
			view.Status = *edit.Status
		}
		if edit.Notes != nil {
			view.Notes = strings.TrimSpace(*edit.Notes)
		}
		view.UpdatedAt = time.Now().UnixMilli()
		_, err = store.db.Exec(`UPDATE submissions SET payload=?,status=?,updated_at=? WHERE id=? AND deleted=0`, string(mustJSON(view.bugReportPayload)), view.Status, view.UpdatedAt, id)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		sendJSON(w, 200, view)
	})
	root.Get("/bug-reports/images/{file}", func(w http.ResponseWriter, req *http.Request) {
		if !ready(w) {
			return
		}
		file := chi.URLParam(req, "file")
		if len(file) != 42 || !validReportID(file[:36]) || (file[36:] != "_1.png" && file[36:] != "_2.png") {
			sendError(w, 404, "Screenshot not found.")
			return
		}
		var exists int
		if err := store.db.QueryRow(`SELECT 1 FROM submissions WHERE id=? AND deleted=0`, file[:36]).Scan(&exists); err != nil {
			sendError(w, 404, "Screenshot not found.")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, req, filepath.Join(store.dir, file))
	})
}

func validateReportImage(data []byte) error {
	if len(data) == 0 || len(data) > reportImageLimit {
		return errors.New("Each screenshot must be no larger than 8 MB.")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 1920 || cfg.Height != 800 {
		return errors.New("Both screenshots must be PNG six-view sheets sized 1920 × 800.")
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		return errors.New("A screenshot is damaged. Please retake both screenshots.")
	}
	return nil
}
