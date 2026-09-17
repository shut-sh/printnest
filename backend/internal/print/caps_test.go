package print

import "testing"

func testCaps() Caps {
	return Caps{
		MediaSupported:        []string{"iso_a4_210x297mm"},
		MediaDefault:          "iso_a4_210x297mm",
		ColorModesSupported:   []string{"color", "monochrome"},
		ScalingSupported:      []string{"none", "fit", "fill"},
		OrientationsSupported: []int{3, 4},
		CopiesSupported:       true,
		PageRangesSupported:   true,
	}
}

func TestValidateOK(t *testing.T) {
	opts := PrintOptions{Copies: 2, PageRanges: "1-2", ColorMode: "color", Orientation: "portrait", Media: "iso_a4_210x297mm", Scaling: "fit"}
	if err := ValidateOptions(testCaps(), opts); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBadScaling(t *testing.T) {
	opts := PrintOptions{Copies: 1, ColorMode: "color", Orientation: "portrait", Media: "iso_a4_210x297mm", Scaling: "fill"}
	caps := testCaps()
	caps.ScalingSupported = []string{"none", "fit"}
	if err := ValidateOptions(caps, opts); err == nil {
		t.Fatal("expected scaling error")
	} else if ve, ok := IsValidationError(err); !ok || ve.Field != "scaling" {
		t.Fatalf("expected ValidationError scaling, got %v", err)
	}
}

func TestValidateBadColor(t *testing.T) {
	opts := PrintOptions{Copies: 1, ColorMode: "color", Orientation: "portrait", Media: "iso_a4_210x297mm", Scaling: "fit"}
	caps := testCaps()
	caps.ColorModesSupported = []string{"monochrome"}
	if err := ValidateOptions(caps, opts); err == nil {
		t.Fatal("expected color error")
	}
}

func TestParsePageRanges(t *testing.T) {
	r, err := ParsePageRanges("1-2,4")
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 2 || r[0] != [2]int{1, 2} || r[1] != [2]int{4, 4} {
		t.Fatalf("bad parse: %v", r)
	}
	if _, err := ParsePageRanges("0-2"); err == nil {
		t.Fatal("expected error for 0-2")
	}
	if _, err := ParsePageRanges("3-2"); err == nil {
		t.Fatal("expected error for 3-2")
	}
}

func TestScalingMap(t *testing.T) {
	if ToIPPScaling("actual") != "none" || ToIPPScaling("fit") != "fit" || ToIPPScaling("fill") != "fill" {
		t.Fatal("scaling map wrong")
	}
	if ToIPPColorMode("bw") != "monochrome" || ToIPPColorMode("color") != "color" {
		t.Fatal("color map wrong")
	}
	if ToIPPOrientation("landscape") != 4 || ToIPPOrientation("portrait") != 3 {
		t.Fatal("orientation map wrong")
	}
}
