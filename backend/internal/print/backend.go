package print

type PrinterInfo struct {
	Name       string   `json:"name"`
	MakeModel  string   `json:"make_model"`
	State      string   `json:"state"`
	Reasons    []string `json:"state_reasons"`
	DeviceURI  string   `json:"device_uri"`
	URI        string   `json:"uri"`
	Accepting  bool     `json:"accepting"`
	Shared     bool     `json:"shared"`
}

type DeviceInfo struct {
	URI       string `json:"uri"`
	MakeModel string `json:"make_model"`
	Info      string `json:"info"`
	Class     string `json:"class"`
}

type Caps struct {
	MediaSupported          []string `json:"media_supported"`
	MediaDefault            string   `json:"media_default"`
	ColorModesSupported     []string `json:"color_mode_supported"`
	ScalingSupported        []string `json:"scaling_supported"`
	OrientationsSupported   []int    `json:"orientation_supported"`
	CopiesSupported         bool     `json:"copies_supported"`
	PageRangesSupported     bool     `json:"page_ranges_supported"`
	DocumentFormats         []string `json:"document_format_supported"`
	PrinterState            string   `json:"printer_state"`
	StateReasons            []string `json:"state_reasons"`
}

type PrintOptions struct {
	JobName     string
	Copies      int
	PageRanges  string
	ColorMode   string
	Orientation string
	Media       string
	Scaling     string
}

type JobState int

const (
	JobPending    JobState = 3
	JobHeld       JobState = 4
	JobProcessing JobState = 5
	JobStopped    JobState = 6
	JobCanceled   JobState = 7
	JobAborted    JobState = 8
	JobCompleted  JobState = 9
)

func (s JobState) String() string {
	switch s {
	case JobPending:
		return "pending"
	case JobHeld:
		return "held"
	case JobProcessing:
		return "processing"
	case JobStopped:
		return "stopped"
	case JobCanceled:
		return "canceled"
	case JobAborted:
		return "aborted"
	case JobCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

type JobInfo struct {
	ID           int
	State        JobState
	StateReasons []string
	PrinterURI   string
	Name         string
}

type PrinterBackend interface {
	ListPrinters() ([]PrinterInfo, error)
	ScanDevices() ([]DeviceInfo, error)
	GetCaps(printer string) (Caps, error)
	AddPrinter(name, uri, model string) error
	Submit(printer, filePath string, opts PrintOptions) (int, error)
	GetJob(cupsJobID int) (JobInfo, error)
	CancelJob(cupsJobID int) error
}

func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, sub := range []string{
		"connection refused", "no such host", "timeout", "connection reset",
		"network is unreachable", "got http code", "dial ", "connect:",
	} {
		if containsFold(s, sub) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	ls, lsub := toLower(s), toLower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
