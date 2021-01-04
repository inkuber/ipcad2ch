package classifier

import (
	"net"
	"testing"
)

func TestShouldClassifyLocalEntry(t *testing.T) {
	e := Entry{
		SrcIP: net.ParseIP("192.168.0.1"),
		DstIP: net.ParseIP("192.168.0.2"),
	}

	cfg := Config{}

	cfg.Users.Users = make(map[string]string)
	cfg.Users.Users["192.168.0.1"] = "1"
	cfg.Users.Users["192.168.0.2"] = "2"

	cfg.Networks.Networks = make(map[string]string)
	cfg.Networks.Networks["192.168.0.0/16"] = "local"

	classifier := NewClassifier(cfg)
	classifier.Classify(&e)

	if e.UserID != "2" {
		t.Errorf("Should classify user")
	}

	if e.Class != "local" {
		t.Errorf("Should classify class")
	}

	if e.Dir != "in" {
		t.Errorf("Should classify direction")
	}
}

func TestShouldClassifyPeeringEntry(t *testing.T) {
	e := Entry{
		SrcIP: net.ParseIP("192.168.0.1"),
		DstIP: net.ParseIP("10.10.0.1"),
	}

	cfg := Config{}

	cfg.Users.Users = make(map[string]string)
	cfg.Users.Users["192.168.0.1"] = "1"

	cfg.Networks.Networks = make(map[string]string)
	cfg.Networks.Networks["192.168.0.0/16"] = "local"
	cfg.Networks.Networks["10.10.0.0/8"] = "peering"

	classifier := NewClassifier(cfg)
	classifier.Classify(&e)

	if e.UserID != "1" {
		t.Errorf("Should classify user")
	}

	if e.Class != "peering" {
		t.Errorf("Should classify class")
	}

	if e.Dir != "out" {
		t.Errorf("Should classify direction")
	}
}

func TestShouldClassifyInternetEntry(t *testing.T) {
	e := Entry{
		SrcIP: net.ParseIP("192.168.0.1"),
		DstIP: net.ParseIP("10.10.0.1"),
	}

	cfg := Config{}

	cfg.Users.Users = make(map[string]string)
	cfg.Users.Users["192.168.0.1"] = "1"

	cfg.Networks.Networks = make(map[string]string)
	cfg.Networks.Networks["192.168.0.0/16"] = "local"

	classifier := NewClassifier(cfg)
	classifier.Classify(&e)

	if e.UserID != "1" {
		t.Errorf("Should classify user")
	}

	if e.Class != "internet" {
		t.Errorf("Should classify class")
	}

	if e.Dir != "out" {
		t.Errorf("Should classify direction")
	}
}

func TestShouldClassifyDirectionEntry(t *testing.T) {
	e := Entry{
		SrcIP: net.ParseIP("10.10.0.1"),
		DstIP: net.ParseIP("192.168.0.1"),
	}

	cfg := Config{}

	cfg.Users.Users = make(map[string]string)
	cfg.Users.Users["192.168.0.1"] = "1"

	cfg.Networks.Networks = make(map[string]string)
	cfg.Networks.Networks["192.168.0.0/16"] = "local"

	classifier := NewClassifier(cfg)
	classifier.Classify(&e)

	if e.UserID != "1" {
		t.Errorf("Should classify user")
	}

	if e.Class != "internet" {
		t.Errorf("Should classify class")
	}

	if e.Dir != "in" {
		t.Errorf("Should classify direction")
	}
}
