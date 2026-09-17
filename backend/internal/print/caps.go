package print

import (
	"errors"
	"fmt"
	"strings"
)

const (
	ScalingNone = "none"
	ScalingFit  = "fit"
	ScalingFill = "fill"
)

const (
	ColorColor = "color"
	ColorMono  = "monochrome"
)

const (
	OrientPortrait  = "portrait"
	OrientLandscape = "landscape"
)

const MediaA4 = "iso_a4_210x297mm"

type ValidationError struct {
	Field     string
	Value     string
	Supported []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("unsupported %s %q (supported: %s)", e.Field, e.Value, strings.Join(e.Supported, ", "))
}

func ToIPPColorMode(color string) string {
	if color == "bw" || color == "mono" || color == "monochrome" || color == "black" {
		return ColorMono
	}
	return ColorColor
}

func ToIPPOrientation(orient string) int {
	if orient == OrientLandscape || orient == "4" {
		return 4
	}
	return 3
}

func FromIPPOrientation(v int) string {
	if v == 4 {
		return OrientLandscape
	}
	return OrientPortrait
}

func ToIPPScaling(ui string) string {
	switch ui {
	case "actual", "none":
		return ScalingNone
	case "fill":
		return ScalingFill
	default:
		return ScalingFit
	}
}

func strSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[strings.ToLower(strings.TrimSpace(s))] = true
	}
	return m
}

func ValidateOptions(caps Caps, opts PrintOptions) error {
	if opts.Copies < 1 {
		return fmt.Errorf("copies must be >= 1")
	}
	if opts.Copies > 1 && !caps.CopiesSupported {
		if len(caps.MediaSupported) > 0 || caps.CopiesSupported == false {
			return &ValidationError{Field: "copies", Value: fmt.Sprint(opts.Copies), Supported: []string{"1"}}
		}
	}
	if mode := ToIPPColorMode(opts.ColorMode); mode != "" {
		if len(caps.ColorModesSupported) > 0 {
			if !strSet(caps.ColorModesSupported)[mode] {
				return &ValidationError{Field: "color", Value: opts.ColorMode, Supported: caps.ColorModesSupported}
			}
		} else if mode != ColorColor && mode != ColorMono {
			return &ValidationError{Field: "color", Value: opts.ColorMode, Supported: []string{ColorColor, ColorMono}}
		}
	}
	scaling := ToIPPScaling(opts.Scaling)
	if len(caps.ScalingSupported) > 0 {
		if !strSet(caps.ScalingSupported)[scaling] {
			return &ValidationError{Field: "scaling", Value: opts.Scaling, Supported: caps.ScalingSupported}
		}
	} else if scaling != ScalingNone && scaling != ScalingFit && scaling != ScalingFill {
		return &ValidationError{Field: "scaling", Value: opts.Scaling, Supported: []string{ScalingNone, ScalingFit, ScalingFill}}
	}
	orient := ToIPPOrientation(opts.Orientation)
	if len(caps.OrientationsSupported) > 0 {
		ok := false
		for _, o := range caps.OrientationsSupported {
			if o == orient {
				ok = true
				break
			}
		}
		if !ok {
			sups := make([]string, 0, len(caps.OrientationsSupported))
			for _, o := range caps.OrientationsSupported {
				sups = append(sups, FromIPPOrientation(o))
			}
			return &ValidationError{Field: "orientation", Value: opts.Orientation, Supported: sups}
		}
	}
	if opts.Media != "" && len(caps.MediaSupported) > 0 {
		if !strSet(caps.MediaSupported)[opts.Media] {
			return &ValidationError{Field: "media", Value: opts.Media, Supported: caps.MediaSupported}
		}
	}
	if opts.PageRanges != "" {
		if !caps.PageRangesSupported {
			return &ValidationError{Field: "page-ranges", Value: opts.PageRanges, Supported: []string{}}
		}
		if _, err := ParsePageRanges(opts.PageRanges); err != nil {
			return err
		}
	}
	return nil
}

func ParsePageRanges(s string) ([][2]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([][2]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("bad page-range %q", s)
		}
		var lo, hi int
		if strings.Contains(p, "-") {
			b := strings.SplitN(p, "-", 2)
			if _, err := fmt.Sscanf(strings.TrimSpace(b[0]), "%d", &lo); err != nil || lo < 1 {
				return nil, fmt.Errorf("bad page-range %q", s)
			}
			if strings.TrimSpace(b[1]) == "" {
				hi = lo
			} else if _, err := fmt.Sscanf(strings.TrimSpace(b[1]), "%d", &hi); err != nil || hi < lo {
				return nil, fmt.Errorf("bad page-range %q", s)
			}
		} else {
			if _, err := fmt.Sscanf(p, "%d", &lo); err != nil || lo < 1 {
				return nil, fmt.Errorf("bad page-range %q", s)
			}
			hi = lo
		}
		out = append(out, [2]int{lo, hi})
	}
	return out, nil
}

func PageRangesToString(ranges [][2]int) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r[0] == r[1] {
			parts = append(parts, fmt.Sprint(r[0]))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r[0], r[1]))
		}
	}
	return strings.Join(parts, ",")
}

func IsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
