package print

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ipp "github.com/phin1x/go-ipp"
)

type IPPBackend struct {
	cups       *ipp.CUPSClient
	client     *ipp.IPPClient
	httpClient *http.Client
	host       string
	port       int
	user       string
	timeout    time.Duration
}

func NewIPPBackend(host string, port int, user string, timeout time.Duration) *IPPBackend {
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	adapter := ipp.NewHttpAdapter(host, port, user, "", false, ipp.WithHttpClient(httpClient))
	return &IPPBackend{
		cups:       ipp.NewCUPSClientWithAdapter(user, adapter),
		client:     ipp.NewIPPClientWithAdapter(user, adapter),
		httpClient: httpClient,
		host:       host,
		port:       port,
		user:       user,
		timeout:    timeout,
	}
}

func (b *IPPBackend) ippClient() *ipp.IPPClient {
	return b.client
}

func attrStr(a ipp.Attributes, key string) string {
	vals, ok := a[key]
	if !ok || len(vals) == 0 {
		return ""
	}
	if s, ok := vals[0].Value.(string); ok {
		return s
	}
	return fmt.Sprint(vals[0].Value)
}

func attrStrs(a ipp.Attributes, key string) []string {
	vals, ok := a[key]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		switch t := v.Value.(type) {
		case string:
			out = append(out, t)
		case []int32:
			for _, n := range t {
				out = append(out, fmt.Sprint(n))
			}
		default:
			out = append(out, fmt.Sprint(v.Value))
		}
	}
	return out
}

func attrInt(a ipp.Attributes, key string) int {
	vals, ok := a[key]
	if !ok || len(vals) == 0 {
		return 0
	}
	switch t := vals[0].Value.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int8:
		return int(t)
	default:
		return 0
	}
}

func attrInts(a ipp.Attributes, key string) []int {
	vals, ok := a[key]
	if !ok {
		return nil
	}
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		switch t := v.Value.(type) {
		case int:
			out = append(out, t)
		case int32:
			out = append(out, int(t))
		case int8:
			out = append(out, int(t))
		case []int32:
			for _, n := range t {
				out = append(out, int(n))
			}
		}
	}
	return out
}

func printerStateString(v int) string {
	switch v {
	case 3:
		return "idle"
	case 4:
		return "processing"
	case 5:
		return "stopped"
	default:
		return fmt.Sprint(v)
	}
}

var capsAttrs = []string{
	"media-supported", "media-default",
	"print-color-mode-supported", "print-color-mode-default",
	"print-scaling-supported", "print-scaling-default",
	"orientation-requested-supported", "orientation-requested-default",
	"copies-supported", "page-ranges-supported",
	"document-format-supported",
	"printer-state", "printer-state-reasons",
	"printer-make-and-model", "printer-uri-supported",
}

func (b *IPPBackend) ListPrinters() ([]PrinterInfo, error) {
	m, err := b.cups.GetPrinters(nil)
	if err != nil {
		return nil, asUnresponsive("", b.timeout, err)
	}
	out := make([]PrinterInfo, 0, len(m))
	for name, a := range m {
		state := printerStateString(attrInt(a, ipp.AttributePrinterState))
		out = append(out, PrinterInfo{
			Name:      name,
			MakeModel: attrStr(a, ipp.AttributePrinterMakeAndModel),
			State:     state,
			Reasons:   attrStrs(a, ipp.AttributePrinterStateReasons),
			DeviceURI: attrStr(a, ipp.AttributeDeviceURI),
			URI:       attrStr(a, ipp.AttributePrinterUriSupported),
			Accepting: true,
			Shared:    false,
		})
		if v, ok := a[ipp.AttributePrinterIsAcceptingJobs]; ok && len(v) > 0 {
			if bv, ok := v[0].Value.(bool); ok {
				out[len(out)-1].Accepting = bv
			}
		}
		if v, ok := a[ipp.AttributePrinterIsShared]; ok && len(v) > 0 {
			if bv, ok := v[0].Value.(bool); ok {
				out[len(out)-1].Shared = bv
			}
		}
	}
	return out, nil
}

func (b *IPPBackend) ScanDevices() ([]DeviceInfo, error) {
	m, err := b.cups.GetDevices()
	if err != nil {
		return nil, asUnresponsive("", b.timeout, err)
	}
	out := make([]DeviceInfo, 0, len(m))
	for uri, a := range m {
		out = append(out, DeviceInfo{
			URI:       uri,
			MakeModel: attrStr(a, "device-make-and-model"),
			Info:      attrStr(a, "device-info"),
			Class:     attrStr(a, "device-class"),
		})
	}
	return out, nil
}

func (b *IPPBackend) GetCaps(printer string) (Caps, error) {
	a, err := b.ippClient().GetPrinterAttributes(printer, capsAttrs)
	if err != nil {
		return Caps{}, asUnresponsive(printer, b.timeout, err)
	}
	c := Caps{
		MediaSupported:        attrStrs(a, "media-supported"),
		MediaDefault:          attrStr(a, "media-default"),
		ColorModesSupported:   attrStrs(a, "print-color-mode-supported"),
		ScalingSupported:      attrStrs(a, "print-scaling-supported"),
		OrientationsSupported: attrInts(a, "orientation-requested-supported"),
		DocumentFormats:       attrStrs(a, "document-format-supported"),
		PrinterState:          printerStateString(attrInt(a, ipp.AttributePrinterState)),
		StateReasons:          attrStrs(a, ipp.AttributePrinterStateReasons),
	}
	if len(c.ColorModesSupported) == 0 {
		c.ColorModesSupported = []string{ColorColor, ColorMono}
	}
	if v, ok := a["copies-supported"]; ok && len(v) > 0 {
		c.CopiesSupported = true
		_ = v
	} else {
		c.CopiesSupported = true
	}
	if v, ok := a["page-ranges-supported"]; ok && len(v) > 0 {
		if bv, ok := v[0].Value.(bool); ok {
			c.PageRangesSupported = bv
		} else {
			c.PageRangesSupported = true
		}
	} else {
		c.PageRangesSupported = true
	}
	if c.MediaDefault == "" {
		c.MediaDefault = MediaA4
	}
	return c, nil
}

func (b *IPPBackend) AddPrinter(name, uri, model string) error {
	name = strings.TrimSpace(name)
	uri = strings.TrimSpace(uri)
	if name == "" || uri == "" {
		return fmt.Errorf("name and uri required")
	}
	if err := b.cups.CreatePrinter(name, uri, "everywhere", true, ipp.ErrorPolicyRetryJob, model, "PrintNest"); err != nil {
		return asUnresponsive(name, b.timeout, err)
	}
	if err := b.cups.AcceptJobs(name); err != nil {
		return asUnresponsive(name, b.timeout, err)
	}
	return nil
}

func (b *IPPBackend) jobAttrs(opts PrintOptions) map[string]any {
	m := map[string]any{}
	if opts.Copies >= 1 {
		m[ipp.AttributeCopies] = opts.Copies
	}
	m[ipp.AttributeMedia] = firstNonEmpty(opts.Media, MediaA4)
	m[ipp.AttributePrintScaling] = ToIPPScaling(opts.Scaling)
	if opts.PageRanges != "" {
		ranges, err := ParsePageRanges(opts.PageRanges)
		if err == nil && len(ranges) > 0 {
			m["page-ranges"] = pageRangesValue(ranges)
		}
	}
	return m
}

type pageRangesValue [][2]int

func (b *IPPBackend) encodeJobPayload(printer, docName string, opts PrintOptions) ([]byte, error) {
	printerURI := fmt.Sprintf("ipp://localhost/printers/%s", printer)
	req := ipp.NewRequest(ipp.OperationPrintJob, 1)
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI
	req.OperationAttributes[ipp.AttributeRequestingUserName] = b.user
	req.OperationAttributes[ipp.AttributeJobName] = docName
	req.OperationAttributes[ipp.AttributeDocumentFormat] = "application/pdf"
	req.OperationAttributes[ipp.AttributeCopies] = firstNonZero(opts.Copies, 1)
	req.JobAttributes[ipp.AttributeMedia] = firstNonEmpty(opts.Media, MediaA4)
	req.JobAttributes[ipp.AttributePrintScaling] = ToIPPScaling(opts.Scaling)
	payload, err := req.Encode()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.PageRanges) == "" {
		return payload, nil
	}
	ranges, err := ParsePageRanges(opts.PageRanges)
	if err != nil || len(ranges) == 0 {
		return payload, nil
	}
	jobGroup := encodeRangeGroup(ranges)
	end := []byte{0x03}
	body := payload[:len(payload)-1]
	out := make([]byte, 0, len(body)+1+len(jobGroup)+1)
	out = append(out, body...)
	if !hasJobGroup(body) {
		out = append(out, 0x02)
	}
	out = append(out, jobGroup...)
	out = append(out, end...)
	return out, nil
}

func hasJobGroup(payload []byte) bool {
	for _, b := range payload {
		if b == 0x02 {
			return true
		}
	}
	return false
}

func encodeRangeGroup(ranges [][2]int) []byte {
	var buf bytes.Buffer
	name := "page-ranges"
	for i, r := range ranges {
		_ = binary.Write(&buf, binary.BigEndian, int8(0x33))
		if i == 0 {
			_ = binary.Write(&buf, binary.BigEndian, int16(len(name)))
			buf.WriteString(name)
		} else {
			_ = binary.Write(&buf, binary.BigEndian, int16(0))
		}
		_ = binary.Write(&buf, binary.BigEndian, int16(8))
		_ = binary.Write(&buf, binary.BigEndian, int32(r[0]))
		_ = binary.Write(&buf, binary.BigEndian, int32(r[1]))
	}
	return buf.Bytes()
}

func (b *IPPBackend) submitRaw(printer string, opts PrintOptions, docName string, file *os.File, size int) (int, error) {
	payload, err := b.encodeJobPayload(printer, docName, opts)
	if err != nil {
		return 0, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	url := fmt.Sprintf("http://%s:%d/printers/%s", b.host, b.port, printer)
	body := io.MultiReader(bytes.NewReader(payload), file)
	httpReq, err := http.NewRequest("POST", url, body)
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", ipp.ContentTypeIPP)
	httpReq.Header.Set("Content-Length", strconv.Itoa(len(payload)+size))
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return 0, asUnresponsive(printer, b.timeout, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, ipp.HTTPError{Code: resp.StatusCode}
	}
	ippResp, err := ipp.NewResponseDecoder(resp.Body).Decode(nil)
	if err != nil {
		return 0, err
	}
	if err := ippResp.CheckForErrors(); err != nil {
		return 0, fmt.Errorf("received error IPP response: %w", err)
	}
	if len(ippResp.JobAttributes) == 0 {
		return 0, fmt.Errorf("server doesn't returned a job id")
	}
	vals := ippResp.JobAttributes[0][ipp.AttributeJobID]
	if len(vals) == 0 {
		return 0, fmt.Errorf("server doesn't returned a job id")
	}
	switch t := vals[0].Value.(type) {
	case int:
		return t, nil
	case int32:
		return int(t), nil
	default:
		return 0, fmt.Errorf("bad job-id type %T", vals[0].Value)
	}
}

func rawRange(_ *ipp.Request, _ string, _ pageRangesValue) {
}

func firstNonZero(n, def int) int {
	if n < 1 {
		return def
	}
	return n
}

func firstNonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (b *IPPBackend) Submit(printer, filePath string, opts PrintOptions) (int, error) {
	caps, err := b.GetCaps(printer)
	if err != nil {
		if IsUnresponsive(err) {
			return 0, err
		}
		log.Printf("printnest: GetCaps failed for %q: %v", printer, err)
	} else if err := ValidateOptions(caps, opts); err != nil {
		return 0, err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	name := opts.JobName
	if name == "" {
		name = filepath.Base(filePath)
	}
	id, err := b.submitRaw(printer, opts, name, f, int(st.Size()))
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (b *IPPBackend) GetJob(cupsJobID int) (JobInfo, error) {
	a, err := b.ippClient().GetJobAttributes(cupsJobID, []string{
		"job-id", "job-state", "job-state-reasons", "job-printer-uri", "job-name",
	})
	if err != nil {
		return JobInfo{}, asUnresponsive("", b.timeout, err)
	}
	state := JobState(attrInt(a, "job-state"))
	return JobInfo{
		ID:           attrInt(a, "job-id"),
		State:        state,
		StateReasons: attrStrs(a, "job-state-reasons"),
		PrinterURI:   attrStr(a, "job-printer-uri"),
		Name:         attrStr(a, "job-name"),
	}, nil
}

func (b *IPPBackend) CancelJob(cupsJobID int) error {
	return asUnresponsive("", b.timeout, b.ippClient().CancelJob(cupsJobID, false))
}
