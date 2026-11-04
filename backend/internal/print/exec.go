package print

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ExecBackend struct {
	CUPSUser string
}

func NewExecBackend(user string) *ExecBackend { return &ExecBackend{CUPSUser: user} }

func (b *ExecBackend) env() []string {
	env := os.Environ()
	if b.CUPSUser != "" {
		env = append(env, "CUPS_USER="+b.CUPSUser)
	}
	return env
}

func run(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("%s: %s", name, msg)
	}
	return out.String(), nil
}

func (b *ExecBackend) ListPrinters() ([]PrinterInfo, error) {
	out, err := run(b.env(), "lpstat", "-p", "-d")
	if err != nil && out == "" {
		return nil, err
	}
	var printers []PrinterInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "printer ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				printers = append(printers, PrinterInfo{Name: fields[1]})
			}
		}
	}
	return printers, nil
}

func (b *ExecBackend) ScanDevices() ([]DeviceInfo, error) {
	out, err := run(b.env(), "lpinfo", "-v")
	if err != nil {
		return nil, err
	}
	var devs []DeviceInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		uri := parts[1]
		info := ""
		if len(parts) > 2 {
			info = strings.Join(parts[2:], " ")
		}
		devs = append(devs, DeviceInfo{URI: uri, Info: info, Class: parts[0]})
	}
	return devs, nil
}

func (b *ExecBackend) GetCaps(printer string) (Caps, error) {
	out, err := run(b.env(), "lpoptions", "-p", printer, "-l")
	if err != nil {
		return Caps{}, err
	}
	c := Caps{
		CopiesSupported:     true,
		PageRangesSupported: true,
		ColorModesSupported: []string{ColorColor, ColorMono},
		ScalingSupported:    []string{ScalingNone, ScalingFit, ScalingFill},
		MediaDefault:        MediaA4,
	}
	for _, line := range strings.Split(out, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "media") && strings.Contains(l, "a4") {
			c.MediaSupported = append(c.MediaSupported, MediaA4)
		}
	}
	if len(c.MediaSupported) == 0 {
		c.MediaSupported = []string{MediaA4}
	}
	return c, nil
}

func (b *ExecBackend) AddPrinter(name, uri, model string) error {
	args := []string{"-p", name, "-E", "-v", uri, "-m", "everywhere", "-o", "printer-error-policy=retry-job"}
	if _, err := run(b.env(), "lpadmin", args...); err != nil {
		return err
	}
	_, err := run(b.env(), "cupsenable", name)
	return err
}

func colorToExec(color string) string {
	if ToIPPColorMode(color) == ColorMono {
		return "Gray"
	}
	return "Color"
}

func (b *ExecBackend) Submit(printer, filePath string, opts PrintOptions) (int, error) {
	media := opts.Media
	if media == "" {
		media = MediaA4
	}
	mediaOpt := "A4"
	if strings.Contains(strings.ToLower(media), "letter") {
		mediaOpt = "Letter"
	}
	copies := opts.Copies
	if copies < 1 {
		copies = 1
	}
	args := []string{
		"-d", printer,
		"-n", strconv.Itoa(copies),
		"-o", "media=" + mediaOpt,
		"-o", "print-color-mode=" + ToIPPColorMode(opts.ColorMode),
		"-o", "ColorModel=" + colorToExec(opts.ColorMode),
		"-o", "print-scaling=" + ToIPPScaling(opts.Scaling),
		"-o", "orientation-requested=" + strconv.Itoa(ToIPPOrientation(opts.Orientation)),
		"-t", ifEmpty(opts.JobName, filepath.Base(filePath)),
	}
	if opts.PageRanges != "" {
		if _, err := ParsePageRanges(opts.PageRanges); err != nil {
			return 0, err
		}
		args = append(args, "-o", "page-ranges="+opts.PageRanges)
	}
	args = append(args, "--", filePath)
	out, err := run(b.env(), "lp", args...)
	if err != nil {
		return 0, err
	}
	id := parseLPJobID(out)
	if id == 0 {
		return 0, fmt.Errorf("lp output did not contain job id: %q", strings.TrimSpace(out))
	}
	log.Printf("printnest: backend=exec-fallback lp submitted job %d on %s", id, printer)
	return id, nil
}

func parseLPJobID(out string) int {
	for _, f := range strings.Fields(out) {
		if strings.Contains(f, "-") {
			parts := strings.Split(f, "-")
			if n, err := strconv.Atoi(strings.Trim(parts[len(parts)-1], ".")); err == nil {
				return n
			}
		}
	}
	return 0
}

func (b *ExecBackend) GetJob(cupsJobID int) (JobInfo, error) {
	out, err := run(b.env(), "lpstat", "-o")
	if err != nil && out == "" {
		return JobInfo{}, err
	}
	needle := "-" + strconv.Itoa(cupsJobID)
	state := JobCompleted
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			state = JobProcessing
			break
		}
	}
	return JobInfo{ID: cupsJobID, State: state}, nil
}

func (b *ExecBackend) CancelJob(cupsJobID int) error {
	_, err := run(b.env(), "cancel", strconv.Itoa(cupsJobID))
	return err
}

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
