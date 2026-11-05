package print

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

func TestSubmitRawUsesContentLength(t *testing.T) {
	var gotCL int64
	var gotTE []string
	var gotBody int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCL = r.ContentLength
		gotTE = r.TransferEncoding
		gotBody, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	doc := []byte("%PDF-1.4 test document")
	path := filepath.Join(t.TempDir(), "print.pdf")
	if err := os.WriteFile(path, doc, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	backend := NewIPPBackend(u.Hostname(), port, "printnest", 5*time.Second)
	_, err = backend.submitRaw("deskjet2700", PrintOptions{
		Copies: 1, Media: MediaA4, Scaling: ScalingFit,
	}, "print.pdf", f, len(doc))
	if err == nil {
		t.Fatal("expected the fake server's 500 to fail the submission")
	}

	if len(gotTE) != 0 {
		t.Fatalf("body must not be chunked, got TransferEncoding %v", gotTE)
	}
	if gotCL <= int64(len(doc)) {
		t.Fatalf("Content-Length %d should include the IPP payload plus %d document bytes", gotCL, len(doc))
	}
	if gotCL != gotBody {
		t.Fatalf("Content-Length %d does not match body length %d", gotCL, gotBody)
	}
}

func TestSubmitRawDecodesResponse(t *testing.T) {
	response := ipp.NewResponse(ipp.StatusOk, 1)
	response.JobAttributes = append(response.JobAttributes, ipp.Attributes{
		ipp.AttributeJobID: []ipp.Attribute{{Tag: ipp.TagInteger, Name: ipp.AttributeJobID, Value: 7}},
	})
	payload, err := response.Encode()
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", ipp.ContentTypeIPP)
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	doc := []byte("%PDF-1.4 test document")
	path := filepath.Join(t.TempDir(), "print.pdf")
	if err := os.WriteFile(path, doc, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	backend := NewIPPBackend(u.Hostname(), port, "printnest", 5*time.Second)
	id, err := backend.submitRaw("deskjet2700", PrintOptions{
		Copies: 1, Media: MediaA4, Scaling: ScalingFit,
	}, "print.pdf", f, len(doc))
	if err != nil {
		t.Fatalf("submitRaw failed: %v", err)
	}
	if id != 7 {
		t.Fatalf("job id = %d, want 7", id)
	}
}
