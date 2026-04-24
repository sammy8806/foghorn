//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestPNGToICO(t *testing.T) {
	icoBytes, err := pngToICO(IconGreen)
	if err != nil {
		t.Fatalf("pngToICO returned error: %v", err)
	}
	if len(icoBytes) <= len(IconGreen) {
		t.Fatalf("expected ICO wrapper around PNG bytes")
	}

	reader := bytes.NewReader(icoBytes)
	var reserved, iconType, count uint16
	for _, target := range []*uint16{&reserved, &iconType, &count} {
		if err := binary.Read(reader, binary.LittleEndian, target); err != nil {
			t.Fatalf("reading ICO header: %v", err)
		}
	}
	if reserved != 0 || iconType != 1 || count != 1 {
		t.Fatalf("unexpected ICO header: reserved=%d type=%d count=%d", reserved, iconType, count)
	}

	if !bytes.HasSuffix(icoBytes, IconGreen) {
		t.Fatalf("expected ICO payload to contain original PNG bytes")
	}
}
