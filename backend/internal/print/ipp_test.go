package print

import (
	"bytes"
	"testing"
	"time"
)

func TestEncodeJobPayloadPageRanges(t *testing.T) {
	b := NewIPPBackend("localhost", 631, "printnest", 25*time.Second)
	payload, err := b.encodeJobPayload("deskjet2700", "t.pdf", PrintOptions{
		Copies: 2, PageRanges: "1-2,4", ColorMode: "color",
		Orientation: "portrait", Media: MediaA4, Scaling: "fit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(payload, []byte("page-ranges")) {
		t.Fatal("payload missing page-ranges")
	}
	idx := bytes.Index(payload, []byte("page-ranges"))
	if idx < 3 {
		t.Fatal("page-ranges too early")
	}
	if payload[idx-3] != 0x33 {
		t.Fatalf("page-ranges not tagged 0x33, context % x", payload[idx-10:idx+12])
	}
	if !bytes.Contains(payload, []byte("print-scaling")) {
		t.Fatal("payload missing print-scaling")
	}
	if !bytes.Contains(payload, []byte(MediaA4)) {
		t.Fatal("payload missing media")
	}
}

func TestEncodeJobPayloadNoRanges(t *testing.T) {
	b := NewIPPBackend("localhost", 631, "printnest", 25*time.Second)
	payload, err := b.encodeJobPayload("deskjet2700", "t.pdf", PrintOptions{
		Copies: 1, ColorMode: "monochrome", Orientation: "landscape", Media: MediaA4, Scaling: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte("page-ranges")) {
		t.Fatal("unexpected page-ranges")
	}
	if payload[len(payload)-1] != 0x03 {
		t.Fatal("payload must end with end tag")
	}
}
