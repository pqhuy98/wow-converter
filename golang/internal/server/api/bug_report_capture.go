package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/server/reportshot"
	"github.com/pqhuy98/wow-converter/internal/server/util"
)

type reportCaptureRequest struct {
	ExportID      string `json:"exportId"`
	Model         string `json:"model"`
	SequenceIndex int    `json:"sequenceIndex"`
}

type preparedReport struct {
	Payload      bugReportPayload `json:"payload"`
	Images       [2]string        `json:"images"`
	Files        [2][]byte        `json:"-"`
	mu           sync.Mutex
	submittedURL string
}

const productionReportServerURL = "https://wow.quangdel.com"

// Destination for quota checks, uploads, and the "view your submission" link.
// BUG_REPORT_SERVER_URL overrides; otherwise local in development, wow.quangdel.com in production builds.
func bugReportServerURL() string {
	if value := os.Getenv("BUG_REPORT_SERVER_URL"); value != "" {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	if config.IsDev() {
		port := 3001
		if v := os.Getenv("PORT"); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				port = p
			}
		}
		return "http://127.0.0.1:" + strconv.Itoa(port)
	}
	return productionReportServerURL
}

func bugReportViewURL(id string) string {
	return bugReportServerURL() + "/bug-reports/" + id
}

func localReportOrigin(w http.ResponseWriter, r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
			sendError(w, 403, "Open the desktop app to prepare a report.")
			return false
		}
	}
	return true
}

func registerReportCapture(r Router, d *Deps, exports *util.JobQueue[exportCharacterRequest, exportCharacterResponse]) {
	if d.Config.IsSharedHosting {
		return
	}
	if receiver, err := url.Parse(bugReportServerURL()); err == nil {
		log.Printf("bug report receiver: %s://%s%s", receiver.Scheme, receiver.Host, receiver.EscapedPath())
	}
	captures := util.NewJobQueue(util.QueueConfig[reportCaptureRequest, *preparedReport]{Concurrency: 1, MaxPendingJobs: 5, JobTTL: 30 * time.Minute, JobTimeout: 8 * time.Minute}, func(job *util.Job[reportCaptureRequest, *preparedReport]) (*preparedReport, error) {
		result := exports.CompletedResult(job.Request.ExportID)
		if result == nil || result.ReportMetadata == nil {
			return nil, errors.New("This export is no longer available. Export the model again to report it.")
		}
		metadata := result.ReportMetadata
		var selected *reportshot.Request
		payload := bugReportPayload{ID: job.ID, WowheadURL: metadata.Request.Character.Base.Value, ModelName: path.Base(job.Request.Model), Request: metadata.Request, GitSHA: metadata.GitSHA, BuildKey: metadata.BuildKey, Product: metadata.Product, ExportedAt: metadata.ExportedAt, Notes: "Preparing screenshots"}
		for _, model := range metadata.Models {
			if model.Path != job.Request.Model {
				continue
			}
			for _, seq := range model.Sequences {
				if seq.Index == job.Request.SequenceIndex {
					payload.Sequence = seq
					selected = &reportshot.Request{BaseURL: fmt.Sprintf("http://127.0.0.1:%d", d.Config.Port), Model: model.Path, Sequence: seq.Name, WowheadURL: payload.WowheadURL, WowSequence: seq.WowName, WowVariant: seq.WowVariant}
				}
			}
		}
		if selected == nil {
			return nil, errors.New("Choose a sequence from the exported model.")
		}
		if err := validateReportPayload(payload); err != nil {
			return nil, err
		}
		if err := checkExportAssets(metadata, d.Config.OutputDir); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(job.Context(), 7*time.Minute)
		defer cancel()
		files, err := reportshot.Capture(ctx, *selected)
		if err != nil {
			return nil, err
		}
		if err = checkExportAssets(metadata, d.Config.OutputDir); err != nil {
			return nil, err
		}
		payload.Notes = ""
		return &preparedReport{Payload: payload, Files: files, Images: [2]string{"/api/bug-report-captures/" + job.ID + "/images/1", "/api/bug-report-captures/" + job.ID + "/images/2"}}, nil
	})
	r.Get("/bug-report-captures/quota", func(w http.ResponseWriter, req *http.Request) {
		setNoStore(w)
		if !localReportOrigin(w, req) {
			return
		}
		proxyReportRequest(w, req, "GET", "/api/bug-reports/quota", nil, "")
	})
	r.Post("/bug-report-captures", func(w http.ResponseWriter, req *http.Request) {
		if !localReportOrigin(w, req) {
			return
		}
		var capture reportCaptureRequest
		if err := readJSONBody(req, &capture); err != nil {
			sendError(w, 400, "Invalid screenshot request.")
			return
		}
		if result := exports.CompletedResult(capture.ExportID); result == nil || result.ReportMetadata == nil {
			sendError(w, 400, "Export this Wowhead model again before reporting it.")
			return
		}
		job := &util.Job[reportCaptureRequest, *preparedReport]{ID: uuid.NewString(), Request: capture, Status: util.JobPending, SubmittedAt: time.Now().UnixMilli()}
		if captures.OutstandingCount() >= 5 {
			sendError(w, 429, "Screenshots are already being prepared. Please wait and try again.")
			return
		}
		captures.AddJob(job)
		sendJSON(w, 202, captures.GetJobStatus(job.ID))
	})
	r.Get("/bug-report-captures/{id}", func(w http.ResponseWriter, req *http.Request) {
		setNoStore(w)
		if !localReportOrigin(w, req) {
			return
		}
		status := captures.GetJobStatus(chi.URLParam(req, "id"))
		if status == nil {
			sendError(w, 404, "Screenshots expired. Please prepare them again.")
			return
		}
		sendJSON(w, 200, status)
	})
	r.Get("/bug-report-captures/{id}/images/{image}", func(w http.ResponseWriter, req *http.Request) {
		setNoStore(w)
		if !localReportOrigin(w, req) {
			return
		}
		result := captures.CompletedResult(chi.URLParam(req, "id"))
		index, err := strconv.Atoi(chi.URLParam(req, "image"))
		if result == nil || *result == nil || err != nil || index < 1 || index > 2 {
			sendError(w, 404, "Screenshot not found. Prepare screenshots again.")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write((*result).Files[index-1])
	})
	r.Post("/bug-report-captures/{id}/submit", func(w http.ResponseWriter, req *http.Request) {
		if !localReportOrigin(w, req) {
			return
		}
		var body struct {
			Notes string `json:"notes"`
		}
		if err := readJSONBody(req, &body); err != nil {
			sendError(w, 400, "Invalid report notes.")
			return
		}
		result := captures.CompletedResult(chi.URLParam(req, "id"))
		if result == nil || *result == nil {
			sendError(w, 400, "Prepare both screenshots before submitting.")
			return
		}
		prepared := *result
		prepared.mu.Lock()
		defer prepared.mu.Unlock()
		if prepared.submittedURL != "" {
			sendJSON(w, 200, map[string]string{"id": prepared.Payload.ID, "url": prepared.submittedURL})
			return
		}
		payload := prepared.Payload
		payload.Notes = body.Notes
		if err := validateReportPayload(payload); err != nil {
			sendError(w, 400, err.Error())
			return
		}
		var buffer bytes.Buffer
		form := multipart.NewWriter(&buffer)
		if err := form.WriteField("report", string(mustJSON(payload))); err != nil {
			sendInternalError(w, err)
			return
		}
		for i, name := range []string{"wowhead", "converter"} {
			part, err := form.CreateFormFile(name, name+".png")
			if err != nil {
				sendInternalError(w, err)
				return
			}
			if _, err = part.Write(prepared.Files[i]); err != nil {
				sendInternalError(w, err)
				return
			}
		}
		if err := form.Close(); err != nil {
			sendInternalError(w, err)
			return
		}
		response, err := remoteReportRequest(req.Context(), "POST", "/api/bug-reports", &buffer, form.FormDataContentType())
		if err != nil {
			sendError(w, 502, "Could not reach the report server. Your screenshots are ready; please try submitting again.")
			return
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			sendError(w, 502, "The report server did not respond. Please try submitting again.")
			return
		}
		if !json.Valid(data) {
			log.Printf("bug report submit: receiver=%s status=%d content-type=%q body=%q", response.Request.URL.Redacted(), response.StatusCode, response.Header.Get("Content-Type"), string(data[:min(len(data), 512)]))
			sendReportReceiverError(w, response)
			return
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			var accepted struct {
				ID        string `json:"id"`
				URL       string `json:"url"`
				Remaining *int   `json:"remaining,omitempty"`
			}
			if json.Unmarshal(data, &accepted) != nil || accepted.ID == "" {
				sendError(w, 502, "The report server did not confirm the submission. Please try again.")
				return
			}
			accepted.URL = bugReportViewURL(accepted.ID)
			prepared.submittedURL = accepted.URL
			data = mustJSON(accepted)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(data)
	})
}

func remoteReportRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, bugReportServerURL()+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return client.Do(req)
}

func proxyReportRequest(w http.ResponseWriter, req *http.Request, method, path string, body io.Reader, contentType string) {
	resp, err := remoteReportRequest(req.Context(), method, path, body, contentType)
	if err != nil {
		sendError(w, 502, "Could not check today's report allowance. Please try again.")
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		sendError(w, 502, "Could not check today's report allowance.")
		return
	}
	if !json.Valid(data) {
		log.Printf("bug report quota: receiver=%s status=%d content-type=%q body=%q", resp.Request.URL.Redacted(), resp.StatusCode, resp.Header.Get("Content-Type"), string(data[:min(len(data), 512)]))
		sendReportReceiverError(w, resp)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(data)
}

// These diagnostics are returned only by desktop capture routes, never report read APIs.
func sendReportReceiverError(w http.ResponseWriter, response *http.Response) {
	sendJSON(w, 502, map[string]any{
		"error":          "The report server is unavailable. Please try again later.",
		"receiver":       response.Request.URL.Scheme + "://" + response.Request.URL.Host,
		"upstreamStatus": response.StatusCode,
	})
}
