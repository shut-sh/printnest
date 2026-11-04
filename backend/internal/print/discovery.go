package print

import (
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
	if IsUnresponsive(err) {
		return nil, err
	}
	if IsTransportError(err) || isIPPError(err) {
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
	if IsUnresponsive(err) {
		return nil, err
	}
	if IsTransportError(err) || isIPPError(err) {
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
	if IsUnresponsive(err) {
		return Caps{}, err
	}
	if IsTransportError(err) || isIPPError(err) {
		log.Printf("printnest: GetCaps IPP failed (%v), backend=exec-fallback", err)
		return b.fallback.GetCaps(printer)
	}
	return Caps{}, err
}

func (b *FailoverBackend) AddPrinter(name, uri, model string) error {
	if err := b.primary.AddPrinter(name, uri, model); err == nil {
		b.cache.mu.Lock()
		b.cache.at = time.Time{}
		b.cache.mu.Unlock()
		return nil
	} else if IsUnresponsive(err) {
		return err
	} else if !(IsTransportError(err) || isIPPError(err)) {
		return err
	} else {
		log.Printf("printnest: AddPrinter IPP failed (%v), backend=exec-fallback", err)
	}
	if err := b.fallback.AddPrinter(name, uri, model); err != nil {
		return err
	}
	b.cache.mu.Lock()
	b.cache.at = time.Time{}
	b.cache.mu.Unlock()
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
	if msg := err.Error(); strings.Contains(msg, "unsupported ") {
		return 0, err
	}
	if !IsTransportError(err) && !isIPPError(err) {
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
	if IsUnresponsive(err) {
		return JobInfo{}, err
	}
	if IsTransportError(err) || isIPPError(err) {
		return b.fallback.GetJob(cupsJobID)
	}
	return JobInfo{}, err
}

func (b *FailoverBackend) CancelJob(cupsJobID int) error {
	if err := b.primary.CancelJob(cupsJobID); err == nil {
		return nil
	} else if IsUnresponsive(err) {
		return err
	} else if !(IsTransportError(err) || isIPPError(err)) {
		return err
	} else {
		log.Printf("printnest: CancelJob IPP failed (%v), backend=exec-fallback", err)
	}
	return b.fallback.CancelJob(cupsJobID)
}

func isIPPError(err error) bool {
	if err == nil {
		return false
	}
	var ippErr ipp.IPPError
	if e, ok := err.(ipp.IPPError); ok {
		ippErr = e
		_ = ippErr
		return false
	}
	var httpErr ipp.HTTPError
	if e, ok := err.(ipp.HTTPError); ok {
		_ = e
		_ = httpErr
		return true
	}
	s := fmt.Sprint(err)
	return strings.Contains(s, "ipp status:") || strings.Contains(s, "got http code")
}
