package convert

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/image/webp"
)

var ErrHEICNoTool = errors.New("heic support requires heif-convert (libheif-examples); file rejected")

const a4W, a4H = 595.28, 841.89

func IsImageMime(mime, name string) bool {
	m := strings.ToLower(mime)
	if strings.HasPrefix(m, "image/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif":
		return true
	}
	return false
}

func IsPDF(mime, name string) bool {
	if strings.EqualFold(mime, "application/pdf") {
		return true
	}
	return strings.EqualFold(filepath.Ext(name), ".pdf")
}

func heifConvert(src, dst string) error {
	if _, err := exec.LookPath("heif-convert"); err != nil {
		return ErrHEICNoTool
	}
	cmd := exec.Command("heif-convert", src, dst)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.TrimSpace(errb.String()) != "" {
			return fmt.Errorf("heif-convert: %s", strings.TrimSpace(errb.String()))
		}
		return fmt.Errorf("heif-convert: %w", err)
	}
	return nil
}

func decodeImage(path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".heic" || ext == ".heif" {
		tmp := path + ".heic.jpg"
		if err := heifConvert(path, tmp); err != nil {
			return nil, err
		}
		defer os.Remove(tmp)
		path = tmp
		ext = ".jpg"
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if ext == ".webp" {
		return webp.Decode(f)
	}
	img, _, err := image.Decode(f)
	if err != nil {
		if strings.Contains(err.Error(), "unknown format") && (ext == ".jpg" || ext == ".jpeg") {
			f2, err2 := os.Open(path)
			if err2 != nil {
				return nil, err
			}
			defer f2.Close()
			return webp.Decode(f2)
		}
		return nil, err
	}
	return img, nil
}

func writePDFHeader(buf *bytes.Buffer, objs []string) []int {
	offsets := make([]int, 0, len(objs)+1)
	pos := buf.Len()
	for _, o := range objs {
		offsets = append(offsets, pos)
		n, _ := buf.WriteString(o)
		pos += n
	}
	return offsets
}

func encodeImagePDF(img image.Image, orientation string) []byte {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw <= 0 || ih <= 0 {
		iw, ih = 100, 100
	}
	landscape := strings.EqualFold(orientation, "landscape")
	pw, ph := a4W, a4H
	if landscape {
		pw, ph = ph, pw
	}
	imgW, imgH := float64(iw), float64(ih)
	if (imgW > imgH) != landscape {
		imgW, imgH = imgH, imgW
	}
	scale := min(pw/imgW, ph/imgH)
	dw, dh := imgW*scale, imgH*scale
	dx, dy := (pw-dw)/2, (ph-dh)/2

	pix := make([]byte, 0, iw*ih*3)
	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			pix = append(pix, byte(r>>8), byte(g>>8), byte(bl>>8))
		}
	}
	raw := []byte(fmt.Sprintf("q\n%.2f 0 0 %.2f %.2f %.2f cm\nBI\n/W %d\n/H %d\n/CS /RGB\n/BPC 8\nID\n", dw, dh, dx, dy, iw, ih))
	raw = append(raw, pix...)
	raw = append(raw, []byte("\nEI\nQ")...)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		fmt.Sprintf("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Contents 4 0 R /Resources << /ProcSet [/PDF /ImageC] >> >>\nendobj\n", pw, ph),
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n", len(raw)),
	}
	offsets := writePDFHeader(&buf, objs)
	buf.Write(raw)
	buf.WriteString("\nendstream\nendobj\n")
	offsets = append(offsets, buf.Len())
	xrefPos := buf.Len()
	n := len(offsets) + 1
	fmt.Fprintf(&buf, "xref\n0 %d\n", n)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[:len(offsets)-1] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", n, xrefPos)
	return buf.Bytes()
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func ImageToPDF(srcPath, orientation string) ([]byte, error) {
	img, err := decodeImage(srcPath)
	if err != nil {
		return nil, err
	}
	return encodeImagePDF(img, orientation), nil
}

func EnsurePrintPDF(srcPath, mime, orientation string) (pdfPath string, cleanup func(), err error) {
	cleanup = func() {}
	if IsPDF(mime, srcPath) {
		return srcPath, cleanup, nil
	}
	if !IsImageMime(mime, srcPath) {
		return "", cleanup, fmt.Errorf("unsupported file type: %s", mime)
	}
	pdf, err := ImageToPDF(srcPath, orientation)
	if err != nil {
		return "", cleanup, err
	}
	dst := srcPath + ".print.pdf"
	if err := os.WriteFile(dst, pdf, 0o600); err != nil {
		return "", cleanup, err
	}
	return dst, func() {
		if dst != srcPath {
			os.Remove(dst)
		}
	}, nil
}
