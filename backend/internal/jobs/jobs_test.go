package jobs

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"printnest/internal/db"
	"printnest/internal/print"
)

type fakeBackend struct {
	job print.JobInfo
	err error
}

func (f fakeBackend) ListPrinters() ([]print.PrinterInfo, error) { return nil, nil }
func (f fakeBackend) ScanDevices() ([]print.DeviceInfo, error)   { return nil, nil }
func (f fakeBackend) GetCaps(string) (print.Caps, error)         { return print.Caps{}, nil }
func (f fakeBackend) AddPrinter(string, string, string) error    { return nil }
func (f fakeBackend) Submit(string, string, print.PrintOptions) (int, error) {
	return 0, nil
}
func (f fakeBackend) GetJob(int) (print.JobInfo, error) { return f.job, f.err }
func (f fakeBackend) CancelJob(int) error               { return nil }

func TestRefreshFlagsStalledJob(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	old := time.Now().Add(-20 * time.Minute).Unix()
	j := db.Job{
		ID:        "j1",
		OrigName:  "a.pdf",
		Mime:      "application/pdf",
		Copies:    1,
		ColorMode: "color",
		Media:     "iso_a4_210x297mm",
		Scaling:   "fit",
		Status:    "pending",
		CUPSJobID: 42,
		CreatedAt: old,
		PrintedAt: old,
	}
	if err := store.InsertJob(j); err != nil {
		t.Fatal(err)
	}

	s := NewStore(store, fakeBackend{job: print.JobInfo{ID: 42, State: print.JobPending}}, t.TempDir(), 1<<20)
	got, err := s.Refresh("j1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Error, "power cycle") {
		t.Fatalf("expected stalled hint, got %q", got.Error)
	}
	if got.Status != "pending" {
		t.Fatalf("status should stay pending, got %q", got.Status)
	}
}

func TestRefreshKeepsFreshJobQuiet(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().Unix()
	if err := store.InsertJob(db.Job{
		ID: "j2", OrigName: "a.pdf", Mime: "application/pdf", Copies: 1,
		ColorMode: "color", Media: "iso_a4_210x297mm", Scaling: "fit",
		Status: "processing", CUPSJobID: 7, CreatedAt: now, PrintedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	s := NewStore(store, fakeBackend{job: print.JobInfo{ID: 7, State: print.JobProcessing}}, t.TempDir(), 1<<20)
	got, err := s.Refresh("j2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Error != "" {
		t.Fatalf("fresh job should have no hint, got %q", got.Error)
	}
}

func TestRefreshFlagsStalledProcessing(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	old := time.Now().Add(-6 * time.Minute).Unix()
	if err := store.InsertJob(db.Job{
		ID: "j3", OrigName: "a.pdf", Mime: "application/pdf", Copies: 1,
		ColorMode: "color", Media: "iso_a4_210x297mm", Scaling: "fit",
		Status: "processing", CUPSJobID: 8, CreatedAt: old, PrintedAt: old,
	}); err != nil {
		t.Fatal(err)
	}

	s := NewStore(store, fakeBackend{job: print.JobInfo{ID: 8, State: print.JobProcessing}}, t.TempDir(), 1<<20)
	got, err := s.Refresh("j3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Error, "power cycle") {
		t.Fatalf("expected stalled hint, got %q", got.Error)
	}
}

func TestRefreshDoesNotFlagQueuedJobTooEarly(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	old := time.Now().Add(-10 * time.Minute).Unix()
	if err := store.InsertJob(db.Job{
		ID: "j4", OrigName: "a.pdf", Mime: "application/pdf", Copies: 1,
		ColorMode: "color", Media: "iso_a4_210x297mm", Scaling: "fit",
		Status: "pending", CUPSJobID: 9, CreatedAt: old, PrintedAt: old,
	}); err != nil {
		t.Fatal(err)
	}

	s := NewStore(store, fakeBackend{job: print.JobInfo{ID: 9, State: print.JobPending}}, t.TempDir(), 1<<20)
	got, err := s.Refresh("j4")
	if err != nil {
		t.Fatal(err)
	}
	if got.Error != "" {
		t.Fatalf("queued job under 15m should be quiet, got %q", got.Error)
	}
}
