package ipcad

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	SrcIP   net.IP
	DstIP   net.IP
	Packets uint64
	Bytes   uint64
	SrcPort uint16
	DstPort uint16
	Proto   uint8
	Iface   string

	Collected time.Time
}

type Config struct {
	Collected string `yaml:"collected"`
	Pipe      bool   `yaml:"pipe"`
}

var (
	done      chan struct{}
	stop      chan struct{}
	collected time.Time
)

func Read(wg *sync.WaitGroup, cfg Config, in io.Reader, out chan *Entry) {
	log.Println("Start ipcad read coroutine")

	defer wg.Done()
	defer close(out)

	var err error
	collected = time.Now()
	if cfg.Collected != "" {
		collected, err = time.Parse(time.RFC3339, cfg.Collected)
		if err != nil {
			log.Fatal(err)
		}
	}

	sent := 0
	index := 0
	scanner := bufio.NewScanner(in)
	for {
		if scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				entry, ok := Parse(line)
				if ok {
					out <- entry

					if cfg.Pipe {
						fmt.Println(line)
					}

					sent = sent + 1
				}
			}
		} else {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			} else {
				break
			}
		}

		if index%100000 == 0 {
			log.Println(fmt.Sprintf("Reading ipcad %d", index))
		}

		index = index + 1
	}

	log.Println(fmt.Sprintf("Sended %d, total %d rows", sent, index))
}

func Parse(line string) (*Entry, bool) {
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
			SrcIP:     srcIP,
			DstIP:     dstIP,
			Packets:   uint64(pkt),
			Bytes:     uint64(bytes),
			SrcPort:   uint16(srcPort),
			DstPort:   uint16(dstPort),
			Proto:     uint8(proto),
			Iface:     iface,
			Collected: collected,
		}, true
	}

	return nil, false
}
