package ipcad

import (
	"fmt"
	"testing"
)

func TestShouldParseIpcadLine(t *testing.T) {
	line := "188.218.183.98   121.82.188.202         1           82  18218   888     8  em1"
	e, ok := Parse(line)
	if !ok {
		t.Errorf("Should be ok")
	}

	if fmt.Sprintf("%v", e.SrcIP) != "188.218.183.98" {
		t.Errorf("Src mismatch")
	}

	if fmt.Sprintf("%v", e.DstIP) != "121.82.188.202" {
		t.Errorf("Dst mismatch")
	}

	if e.Packets != 1 {
		t.Errorf("Pkt mismatch")
	}

	if e.Bytes != 82 {
		t.Errorf("Bytes mismatch")
	}

	if e.SrcPort != 18218 {
		t.Errorf("SrcPort mismatch")
	}

	if e.DstPort != 888 {
		t.Errorf("DstPort mismatch")
	}

	if e.Proto != 8 {
		t.Errorf("Proto mismatch")
	}

	if e.Iface != "em1" {
		t.Errorf("Iface mismatch")
	}
}
