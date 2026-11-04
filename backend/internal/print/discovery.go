package print

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

type discoveryCache struct {
	mu       sync.Mutex
	at       time.Time
	printers []PrinterInfo
}

type FailoverBackend struct {
	primary  *IPPBackend
	fallback *ExecBackend
	cache    discoveryCache
}

func NewFailoverBackend(host string, port int, user string, timeout time.Duration) *FailoverBackend {
	return &FailoverBackend{
		primary:  NewIPPBackend(host, port, user, timeout),
		fallback: NewExecBackend(user),
	}
}

// fallbackAllowed reports whether an IPP failure is a CUPS transport problem
// that the exec tools can work around. An unresponsive printer is not: the
// printer is wedged and retrying through CUPS will stall the same way.
func fallbackAllowed(err error) bool {
	return !IsUnresponsive(err) && (IsTransportError(err) || isIPPError(err))
}

func (b *FailoverBackend) invalidateCache() {
	b.cache.mu.Lock()
	b.cache.at = time.Time{}
	b.cache.printers = nil
	b.cache.mu.Unlock()
}

func (b *FailoverBackend) ListPrinters() ([]PrinterInfo, error) {
	b.cache.mu.Lock()
	if time.Since(b.cache.at) < 30*time.Second && b.cache.printers != nil {
		out := b.cache.printers
		b.cache.mu.Unlock()
		return out, nil
	}
	b.cache.mu.Unlock()

	printers, err := b.primary.ListPrinters()
	if err == nil {
		b.cache.mu.Lock()
		b.cache.at = time.Now()
		b.cache.printers = printers
		b.cache.mu.Unlock()
		return printers, nil
	}
	if fallbackAllowed(err) {
		log.Printf("printnest: ListPrinters IPP failed (%v), backend=exec-fallback", err)
		return b.fallback.ListPrinters()
	}
	return nil, err
}

func (b *FailoverBackend) ScanDevices() ([]DeviceInfo, error) {
	devs, err := b.primary.ScanDevices()
	if err == nil {
		return devs, nil
	}
	if fallbackAllowed(err) {
		log.Printf("printnest: ScanDevices IPP failed (%v), backend=exec-fallback", err)
		return b.fallback.ScanDevices()
	}
	return nil, err
}

func (b *FailoverBackend) GetCaps(printer string) (Caps, error) {
	c, err := b.primary.GetCaps(printer)
	if err == nil {
		return c, nil
	}
	if fallbackAllowed(err) {
		log.Printf("printnest: GetCaps IPP failed (%v), backend=exec-fallback", err)
		return b.fallback.GetCaps(printer)
	}
	return Caps{}, err
}

func (b *FailoverBackend) AddPrinter(name, uri, model string) error {
	err := b.primary.AddPrinter(name, uri, model)
	if err == nil {
		b.invalidateCache()
		return nil
	}
	if !fallbackAllowed(err) {
		return err
	}
	log.Printf("printnest: AddPrinter IPP failed (%v), backend=exec-fallback", err)
	if err := b.fallback.AddPrinter(name, uri, model); err != nil {
		return err
	}
	b.invalidateCache()
	return nil
}

func (b *FailoverBackend) Submit(printer, filePath string, opts PrintOptions) (int, error) {
	id, err := b.primary.Submit(printer, filePath, opts)
	if err == nil {
		return id, nil
	}
	if _, ok := IsValidationError(err); ok {
		return 0, err
	}
	if IsUnresponsive(err) {
		return 0, err
	}
	if strings.Contains(err.Error(), "unsupported ") {
		return 0, err
	}
	if !fallbackAllowed(err) {
		return 0, err
	}
	log.Printf("printnest: Submit IPP failed (%v), backend=exec-fallback", err)
	return b.fallback.Submit(printer, filePath, opts)
}

func (b *FailoverBackend) GetJob(cupsJobID int) (JobInfo, error) {
	info, err := b.primary.GetJob(cupsJobID)
	if err == nil {
		return info, nil
	}
	if fallbackAllowed(err) {
		return b.fallback.GetJob(cupsJobID)
	}
	return JobInfo{}, err
}

func (b *FailoverBackend) CancelJob(cupsJobID int) error {
	err := b.primary.CancelJob(cupsJobID)
	if err == nil {
		return nil
	}
	if !fallbackAllowed(err) {
		return err
	}
	log.Printf("printnest: CancelJob IPP failed (%v), backend=exec-fallback", err)
	return b.fallback.CancelJob(cupsJobID)
}

func isIPPError(err error) bool {
	if err == nil {
		return false
	}
	var ippErr ipp.IPPError
	if errors.As(err, &ippErr) {
		return false
	}
	var httpErr ipp.HTTPError
	if errors.As(err, &httpErr) {
		return true
	}
	s := fmt.Sprint(err)
	return strings.Contains(s, "ipp status:") || strings.Contains(s, "got http code")
}
