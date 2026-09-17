package convert

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	p := filepath.Join(t.TempDir(), "in.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := encodePNG(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImageToPDFPortrait(t *testing.T) {
	p := writePNG(t, 20, 40)
	pdf, err := ImageToPDF(p, "portrait")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("not a pdf")
	}
	if !bytes.Contains(pdf, []byte("/MediaBox [0 0 595.28 841.89]")) {
		t.Fatal("portrait mediabox wrong")
	}
}

func TestImageToPDFLandscape(t *testing.T) {
	p := writePNG(t, 20, 40)
	pdf, err := ImageToPDF(p, "landscape")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("/MediaBox [0 0 841.89 595.28]")) {
		t.Fatal("landscape mediabox wrong")
	}
}
