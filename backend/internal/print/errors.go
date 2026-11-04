package print

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// UnresponsiveError reports that the printer (via CUPS) accepted the request
// but never answered in time. This is the "printer is on but wedged" case:
// retrying or falling back will not help, the printer needs a power cycle.
type UnresponsiveError struct {
	Printer string
	Timeout time.Duration
	Err     error
}

func (e *UnresponsiveError) Error() string {
	subject := "printer"
	if e.Printer != "" {
		subject = fmt.Sprintf("printer %q", e.Printer)
	}
	return fmt.Sprintf(
		"%s did not respond within %s (CUPS request timed out) · power cycle the printer and retry",
		subject, e.Timeout,
	)
}

func (e *UnresponsiveError) Unwrap() error { return e.Err }

func IsUnresponsive(err error) bool {
	var target *UnresponsiveError
	return errors.As(err, &target)
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, sub := range []string{
		"client.timeout exceeded",
		"context deadline exceeded",
		"i/o timeout",
		"timeout awaiting response headers",
	} {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func asUnresponsive(printer string, timeout time.Duration, err error) error {
	if err == nil || !isTimeoutErr(err) || IsUnresponsive(err) {
		return err
	}
	return &UnresponsiveError{Printer: printer, Timeout: timeout, Err: err}
}
