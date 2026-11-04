package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"printnest/internal/convert"
	"printnest/internal/db"
	"printnest/internal/print"
)

type Store struct {
	DB       *db.DB
	Backend  print.PrinterBackend
	DataDir  string
	MaxBytes int64
}

func NewStore(d *db.DB, b print.PrinterBackend, dataDir string, maxBytes int64) *Store {
	return &Store{DB: d, Backend: b, DataDir: dataDir, MaxBytes: maxBytes}
}

func (s *Store) dir(id string) string {
	return filepath.Join(s.DataDir, "uploads", id)
}

func (s *Store) OriginalPath(id string) string {
	matches, _ := filepath.Glob(filepath.Join(s.dir(id), "original*"))
	if len(matches) > 0 {
		return matches[0]
	}
	return filepath.Join(s.dir(id), "original")
}

func (s *Store) PrintPath(id string) string {
	return filepath.Join(s.dir(id), "print.pdf")
}

func (s *Store) Create(origName, mime string, size int64, data []byte) (db.Job, error) {
	if size > s.MaxBytes || int64(len(data)) > s.MaxBytes {
		return db.Job{}, fmt.Errorf("file too large (max %d MB)", s.MaxBytes>>20)
	}
	if !convert.IsPDF(mime, origName) && !convert.IsImageMime(mime, origName) {
		return db.Job{}, fmt.Errorf("unsupported file type: %s (pdf, jpg, png, webp, heic only)", mime)
	}
	id := uuid.NewString()
	dir := s.dir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return db.Job{}, err
	}
	origPath := filepath.Join(dir, "original"+filepath.Ext(origName))
	if err := os.WriteFile(origPath, data, 0o600); err != nil {
		os.RemoveAll(dir)
		return db.Job{}, err
	}
	j := db.Job{
		ID:          id,
		OrigName:    origName,
		Mime:        mime,
		SizeBytes:   int64(len(data)),
		Copies:      1,
		ColorMode:   "color",
		Orientation: "portrait",
		Media:       print.MediaA4,
		Scaling:     "fit",
		Status:      "uploaded",
		CreatedAt:   time.Now().Unix(),
	}
	if err := s.DB.InsertJob(j); err != nil {
		os.RemoveAll(dir)
		return db.Job{}, err
	}
	return j, nil
}

type PrintRequest struct {
	Printer     string `json:"printer"`
	Copies      int    `json:"copies"`
	PageRanges  string `json:"pageRanges"`
	Color       string `json:"color"`
	Orientation string `json:"orientation"`
	Media       string `json:"media"`
	Scaling     string `json:"scaling"`
}

func (s *Store) SubmitPrint(id string, req PrintRequest) (db.Job, error) {
	j, err := s.DB.GetJob(id)
	if err != nil {
		return db.Job{}, fmt.Errorf("job not found")
	}
	if req.Printer == "" {
		return db.Job{}, fmt.Errorf("printer required")
	}
	if req.Copies >= 1 {
		j.Copies = req.Copies
	} else {
		j.Copies = 1
	}
	j.Printer = req.Printer
	j.PageRanges = req.PageRanges
	if req.Color != "" {
		j.ColorMode = req.Color
	}
	if req.Orientation != "" {
		j.Orientation = req.Orientation
	}
	if req.Media != "" {
		j.Media = req.Media
	}
	if req.Scaling != "" {
		j.Scaling = req.Scaling
	}

	origPath := s.OriginalPath(id)
	pdfPath, cleanup, err := convert.EnsurePrintPDF(origPath, j.Mime, j.Orientation)
	if err != nil {
		j.Status = "failed"
		j.Error = err.Error()
		s.DB.UpdateJob(j)
		return j, err
	}
	defer cleanup()
	if pdfPath != origPath {
		dst := s.PrintPath(id)
		data, err := os.ReadFile(pdfPath)
		if err != nil {
			j.Status = "failed"
			j.Error = err.Error()
			s.DB.UpdateJob(j)
			return j, err
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			j.Status = "failed"
			j.Error = err.Error()
			s.DB.UpdateJob(j)
			return j, err
		}
		pdfPath = dst
	}

	opts := print.PrintOptions{
		JobName:     j.OrigName,
		Copies:      j.Copies,
		PageRanges:  j.PageRanges,
		ColorMode:   j.ColorMode,
		Orientation: j.Orientation,
		Media:       j.Media,
		Scaling:     j.Scaling,
	}
	cupsID, err := s.Backend.Submit(j.Printer, pdfPath, opts)
	if err != nil {
		j.Status = "failed"
		j.Error = describeBackendError(j.Printer, err)
		s.DB.UpdateJob(j)
		return j, err
	}
	j.Status = "pending"
	j.CUPSJobID = cupsID
	j.Error = ""
	j.PrintedAt = time.Now().Unix()
	s.DB.UpdateJob(j)
	return j, nil
}

const (
	stalledProcessingAfter = 5 * time.Minute
	stalledPendingAfter    = 15 * time.Minute
)

func describeBackendError(printer string, err error) string {
	if print.IsUnresponsive(err) {
		return err.Error()
	}
	if ve, ok := print.IsValidationError(err); ok {
		return ve.Error()
	}
	return fmt.Sprintf("printer %q: %v", printer, err)
}

func (s *Store) Refresh(id string) (db.Job, error) {
	j, err := s.DB.GetJob(id)
	if err != nil {
		return db.Job{}, err
	}
	if j.CUPSJobID == 0 || j.Status == "uploaded" {
		return j, nil
	}
	if j.Status == "done" || j.Status == "canceled" {
		return j, nil
	}
	info, err := s.Backend.GetJob(j.CUPSJobID)
	if err != nil {
		if print.IsUnresponsive(err) {
			j.Error = err.Error()
			s.DB.UpdateJob(j)
		}
		return j, nil
	}
	previous := j.Status
	switch info.State {
	case print.JobCompleted:
		j.Status = "done"
	case print.JobCanceled, print.JobAborted:
		j.Status = "canceled"
	case print.JobPending, print.JobHeld:
		j.Status = "pending"
	default:
		j.Status = "processing"
	}
	if j.Status != previous {
		j.PrintedAt = time.Now().Unix()
	}

	reasons := joinReasons(info.StateReasons)
	if j.Status == "done" || j.Status == "canceled" {
		j.Error = reasons
		s.DB.UpdateJob(j)
		return j, nil
	}
	if elapsed, stalled := stalledFor(j); stalled {
		hint := fmt.Sprintf("no progress for %s", elapsed.Round(time.Minute))
		if warning := s.printerWarning(j.Printer); warning != "" {
			hint += " · " + warning
		} else {
			hint += " · check the printer; if it is on but unresponsive, power cycle it"
		}
		if reasons == "" {
			reasons = hint
		} else {
			reasons += " · " + hint
		}
	}
	j.Error = reasons
	s.DB.UpdateJob(j)
	return j, nil
}

func stalledFor(j db.Job) (time.Duration, bool) {
	if j.PrintedAt == 0 {
		return 0, false
	}
	elapsed := time.Since(time.Unix(j.PrintedAt, 0))
	switch j.Status {
	case "processing":
		return elapsed, elapsed > stalledProcessingAfter
	case "pending":
		return elapsed, elapsed > stalledPendingAfter
	default:
		return elapsed, false
	}
}

func (s *Store) printerWarning(printer string) string {
	if printer == "" {
		return ""
	}
	list, err := s.Backend.ListPrinters()
	if err != nil {
		return ""
	}
	for _, p := range list {
		if p.Name != printer {
			continue
		}
		if p.State == "stopped" {
			return "printer is stopped"
		}
		for _, reason := range p.Reasons {
			r := strings.ToLower(reason)
			if strings.Contains(r, "connecting-to-device") ||
				strings.Contains(r, "offline") ||
				strings.Contains(r, "not-responding") {
				return "printer is not reachable"
			}
		}
	}
	return ""
}

func joinReasons(r []string) string {
	benign := map[string]bool{
		"none":                       true,
		"job-incoming":               true,
		"job-queued":                 true,
		"job-printing":               true,
		"job-processing":             true,
		"job-completed-successfully": true,
	}
	out := ""
	for i, s := range r {
		if s == "" || benign[s] {
			continue
		}
		if i > 0 && out != "" {
			out += "; "
		}
		out += s
	}
	return out
}

func (s *Store) Cancel(id string) (db.Job, error) {
	j, err := s.DB.GetJob(id)
	if err != nil {
		return db.Job{}, err
	}
	if j.Status != "pending" && j.Status != "processing" {
		return db.Job{}, fmt.Errorf("job not cancelable in status %q", j.Status)
	}
	if j.CUPSJobID != 0 {
		if err := s.Backend.CancelJob(j.CUPSJobID); err != nil {
			return j, err
		}
	}
	j.Status = "canceled"
	s.DB.UpdateJob(j)
	return j, nil
}

func (s *Store) DeleteFiles(id string) error {
	j, err := s.DB.GetJob(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(s.dir(id)); err != nil {
		return err
	}
	j.FileDeletedAt = time.Now().Unix()
	return s.DB.UpdateJob(j)
}
