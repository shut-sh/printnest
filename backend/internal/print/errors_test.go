package print

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIsTimeoutErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline", errors.New("context deadline exceeded"), true},
		{"client timeout", &url.Error{Op: "Post", URL: "http://x", Err: errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers)")}, true},
		{"net timeout", &net.OpError{Op: "dial", Err: &timeoutErr{}}, true},
		{"refused", errors.New("dial tcp 127.0.0.1:631: connect: connection refused"), false},
		{"ipp status", errors.New("ipp status: client-error-not-found"), false},
	}
	for _, c := range cases {
		if got := isTimeoutErr(c.err); got != c.want {
			t.Errorf("%s: isTimeoutErr = %v, want %v", c.name, got, c.want)
		}
	}
}

type timeoutErr struct{}

func (e *timeoutErr) Error() string   { return "i/o timeout" }
func (e *timeoutErr) Timeout() bool   { return true }
func (e *timeoutErr) Temporary() bool { return true }

func TestUnresponsiveError(t *testing.T) {
	err := asUnresponsive("deskjet2700", 25*time.Second, errors.New("context deadline exceeded"))
	if !IsUnresponsive(err) {
		t.Fatal("expected UnresponsiveError")
	}
	msg := err.Error()
	for _, want := range []string{"deskjet2700", "25s", "power cycle"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q missing %q", msg, want)
		}
	}
}

func TestAsUnresponsiveIgnoresOtherErrors(t *testing.T) {
	src := errors.New("connection refused")
	if err := asUnresponsive("p", time.Second, src); IsUnresponsive(err) || err != src {
		t.Fatalf("non-timeout error must pass through unchanged, got %v", err)
	}
}

func TestIPPBackendTimesOutWhenCUPSStalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
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

	backend := NewIPPBackend(u.Hostname(), port, "printnest", 150*time.Millisecond)
	start := time.Now()
	_, err = backend.GetCaps("deskjet2700")
	if !IsUnresponsive(err) {
		t.Fatalf("expected unresponsive error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout not applied, took %s", elapsed)
	}
}
