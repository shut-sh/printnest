package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"printnest/internal/auth"
	"printnest/internal/convert"
	"printnest/internal/db"
	"printnest/internal/jobs"
	"printnest/internal/print"
)

type Server struct {
	DB       *db.DB
	Jobs     *jobs.Store
	Backend  print.PrinterBackend
	MaxBytes int64
	DefRet   int
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	data, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	json.Unmarshal(data, &body)
	if body.Password == "" {
		body.Password = r.FormValue("password")
	}
	if !auth.CheckPassword(s.DB, body.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid password")
		return
	}
	token, exp, err := auth.CreateSession(s.DB, 30*24*time.Hour)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session failed")
		return
	}
	auth.SetCookie(w, token, exp)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		auth.DestroySession(s.DB, c.Value)
	}
	auth.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := s.Backend.ListPrinters()
	if err != nil {
		writePrinterFailure(w, err)
		return
	}
	if list == nil {
		list = []print.PrinterInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"printers": list})
}

func printerErr(err error) string { return "CUPS: " + err.Error() }

func writePrinterFailure(w http.ResponseWriter, err error) {
	if print.IsUnresponsive(err) {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{"error": err.Error(), "hint": "power-cycle"})
		return
	}
	writeErr(w, http.StatusBadGateway, printerErr(err))
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	devs, err := s.Backend.ScanDevices()
	if err != nil {
		writePrinterFailure(w, err)
		return
	}
	if devs == nil {
		devs = []print.DeviceInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devs})
}

func (s *Server) handleAddPrinter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Name  string `json:"name"`
		URI   string `json:"uri"`
		Model string `json:"model"`
	}
	data, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := json.Unmarshal(data, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" || body.URI == "" {
		writeErr(w, http.StatusBadRequest, "name and uri required")
		return
	}
	if err := s.Backend.AddPrinter(body.Name, body.URI, body.Model); err != nil {
		writePrinterFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) routePrinters(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/printers")
	if rest == "" || rest == "/" {
		s.handlePrinters(w, r)
		return
	}
	if rest == "/discover" {
		s.handleDiscover(w, r)
		return
	}
	if rest == "/add" {
		s.handleAddPrinter(w, r)
		return
	}
	if strings.HasSuffix(rest, "/caps") && r.Method == http.MethodGet {
		name := strings.TrimSuffix(strings.TrimPrefix(rest, "/"), "/caps")
		caps, err := s.Backend.GetCaps(name)
		if err != nil {
			writePrinterFailure(w, err)
			return
		}
		writeJSON(w, http.StatusOK, caps)
		return
	}
	writeErr(w, http.StatusNotFound, "not found")
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxBytes+10<<20)
	if err := r.ParseMultipartForm(s.MaxBytes + 1<<20); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large (max 50 MB)")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file field required")
		return
	}
	defer f.Close()
	if hdr.Size > s.MaxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large (max 50 MB)")
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, s.MaxBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read failed")
		return
	}
	if int64(len(data)) > s.MaxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large (max 50 MB)")
		return
	}
	mime := hdr.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	j, err := s.Jobs.Create(hdr.Filename, mime, hdr.Size, data)
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		if strings.Contains(err.Error(), "unsupported") {
			if convert.IsImageMime(mime, hdr.Filename) && strings.Contains(strings.ToLower(hdr.Filename), ".heic") {
				writeErr(w, 415, err.Error())
				return
			}
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, jobJSON(j))
}

func jobJSON(j db.Job) map[string]any {
	return map[string]any{
		"id": j.ID, "printer": j.Printer, "orig_name": j.OrigName,
		"mime": j.Mime, "copies": j.Copies, "page_ranges": j.PageRanges,
		"color": j.ColorMode, "orientation": j.Orientation, "media": j.Media,
		"scaling": j.Scaling, "status": j.Status, "cups_job_id": j.CUPSJobID,
		"error": j.Error, "created_at": j.CreatedAt,
	}
}

func (s *Server) routeJobs(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/jobs")
	if rest == "" || rest == "/" {
		if r.Method == http.MethodGet {
			limit := 50
			if v := r.URL.Query().Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					limit = n
				}
			}
			list, err := s.DB.ListJobs(limit)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "db failed")
				return
			}
			out := make([]any, 0, len(list))
			for _, j := range list {
				out = append(out, jobJSON(j))
			}
			writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
			return
		}
		if r.Method == http.MethodPost {
			s.handleCreateJob(w, r)
			return
		}
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		j, err := s.Jobs.Refresh(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, "job not found")
			return
		}
		writeJSON(w, http.StatusOK, jobJSON(j))
		return
	}
	if len(parts) == 2 && parts[1] == "print" && r.Method == http.MethodPost {
		var req jobs.PrintRequest
		data, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := json.Unmarshal(data, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		j, err := s.Jobs.SubmitPrint(id, req)
		if err != nil {
			if print.IsUnresponsive(err) {
				writePrinterFailure(w, err)
				return
			}
			if ve, ok := print.IsValidationError(err); ok {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": ve.Error(), "supported": ve.Supported, "field": ve.Field})
				return
			}
			msg := err.Error()
			if strings.Contains(msg, "unsupported") || strings.Contains(msg, "copies") || strings.Contains(msg, "page-range") {
				writeErr(w, http.StatusBadRequest, msg)
				return
			}
			if errors_IsHEIC(err) {
				writeErr(w, 415, msg)
				return
			}
			if strings.Contains(msg, "printer") || strings.Contains(msg, "CUPS") || strings.Contains(msg, "ipp") {
				writeErr(w, http.StatusBadGateway, msg)
				return
			}
			writeErr(w, http.StatusBadRequest, msg)
			return
		}
		writeJSON(w, http.StatusOK, jobJSON(j))
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		j, err := s.Jobs.Cancel(id)
		if err != nil {
			if strings.Contains(err.Error(), "not cancelable") {
				writeErr(w, http.StatusConflict, err.Error())
				return
			}
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, jobJSON(j))
		return
	}
	writeErr(w, http.StatusNotFound, "not found")
}

func errors_IsHEIC(err error) bool {
	return err != nil && strings.Contains(err.Error(), "heif-convert")
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"file_retention_minutes": s.DB.RetentionMinutes(s.DefRet),
		})
	case http.MethodPut:
		var body struct {
			FileRetentionMinutes *int `json:"file_retention_minutes"`
		}
		data, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := json.Unmarshal(data, &body); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if body.FileRetentionMinutes == nil || *body.FileRetentionMinutes <= 0 {
			writeErr(w, http.StatusBadRequest, "file_retention_minutes must be > 0")
			return
		}
		if err := s.DB.SetSetting("file_retention_minutes", strconv.Itoa(*body.FileRetentionMinutes)); err != nil {
			writeErr(w, http.StatusInternalServerError, "db failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"file_retention_minutes": *body.FileRetentionMinutes})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	authed := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, auth.RequireAuth(s.DB, h))
	}
	authed("/api/printers", s.routePrinters)
	authed("/api/printers/", s.routePrinters)
	authed("/api/jobs", s.routeJobs)
	authed("/api/jobs/", s.routeJobs)
	authed("/api/settings", s.handleSettings)
}
