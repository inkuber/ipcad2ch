package ipcad

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
)

type Entry struct {
	Src     net.IP
	Dst     net.IP
	Pkt     uint64
	Bytes   uint64
	SrcPort uint32
	DstPort uint32
	Proto   uint8
	Iface   string
}

var (
	done chan struct{}
	stop chan struct{}
)

func Read(ctx context.Context, in io.Reader, out chan Entry) error {

	defer close(out)

	scanner := bufio.NewScanner(in)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if scanner.Scan() {
				entry, ok := parse(scanner.Text())
				if ok {
					out <- *entry
				}
			} else {
				if err := scanner.Err(); err != nil {
					return err
				} else {
					return nil
				}
			}
		}
	}
}

func parse(line string) (*Entry, bool) {
	fields := strings.Fields(line)

	if len(fields) == 8 {
		if fields[0] == "Source" {
			return nil, false
		}

		srcIP := net.ParseIP(fields[0])
		dstIP := net.ParseIP(fields[1])

		pkt, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, false
		}

		bytes, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, false
		}

		srcPort, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, false
		}

		dstPort, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, false
		}

		proto, err := strconv.Atoi(fields[6])
		if err != nil {
			return nil, false
		}

		iface := fields[7]

		return &Entry{
			Src:     srcIP,
			Dst:     dstIP,
			Pkt:     uint64(pkt),
			Bytes:   uint64(bytes),
			SrcPort: uint32(srcPort),
			DstPort: uint32(dstPort),
			Proto:   uint8(proto),
			Iface:   iface,
		}, true
	}

	return nil, false
}
