package classifier

import (
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

/*
Config struct used in classifier consturctor NewClassifier

Config {
  Users: {
    Fetch {
      URL: URL to fetch users, formats: json, csv

      File: File path to users file, formats: json, csv

      IDField: ID field index for csv
      CIDRField: CIDR field index for csv
      Comma: Field delimiter
    }
    Users: users hash map ip => id
  }
  Networks: {
    Fetch {
      URL: URL to fetch networks, formats: json, csv

      File: File path to networks file, formats: json, csv

      IDField: ID field index for csv
      CIDRField: CIDR field index for csv
      Comma: Field delimiter
    }
    Networks: networks hash map cidr => class, classes: local, peering
  }
}
*/
type Config struct {
	Users struct {
		Fetch struct {
			URL       string `mapstructure:"url"`
			File      string `mapstructure:"file"`
			IDField   int    `mapstructure:"IDField"`
			CIDRField int    `mapstructure:"IPField"`
			Comma     string `mapstructure:"Comma"`
		}

		Users map[string]string `mapstructure:"users"`
	}

	Networks struct {
		Fetch struct {
			URL        string `mapstructure:"url"`
			File       string `mapstructure:"file"`
			CIDRField  int    `mapstructure:"CIDRField"`
			ClassField int    `mapstructure:"classField"`
			Comma      string `mapstructure:"Comma"`
		}

		Networks map[string]string `mapstructure:"networks"`
	}
}

// Entry is DTO object for classification
type Entry struct {
	SrcIP     net.IP
	DstIP     net.IP
	Packets   uint64
	Bytes     uint64
	SrcPort   uint16
	DstPort   uint16
	Proto     uint8
	Iface     string
	Collected time.Time

	UserID string
	Dir    string
	Class  string
}

/*
Classifier class struct

Should be instantiate with NewClassifier method
*/
type Classifier struct {
	Config    Config
	Users     map[uint32]string
	Local     []net.IPNet
	Peering   []net.IPNet
	Multicast net.IPNet
}

var (
	// UNKNOWN dir or class
	UNKNOWN string = "unknown"

	// IN direction
	IN string = "in"

	// OUT direction
	OUT string = "out"

	// LOCAL network
	LOCAL string = "local"

	// PEERING network
	PEERING string = "peering"

	// INTERNET network
	INTERNET string = "internet"

	// MULTICAST network
	MULTICAST string = "multicast"
)

// NewClassifier constructor method
func NewClassifier(cfg Config) Classifier {
	_, mcast, _ := net.ParseCIDR("224.0.0.0/4")

	if cfg.Users.Users == nil {
		cfg.Users.Users = make(map[string]string)
	}

	if cfg.Networks.Networks == nil {
		cfg.Networks.Networks = make(map[string]string)
	}

	c := Classifier{
		Config:    cfg,
		Users:     make(map[uint32]string),
		Local:     make([]net.IPNet, 0),
		Peering:   make([]net.IPNet, 0),
		Multicast: *mcast,
	}

	if cfg.Users.Fetch.URL != "" {
		c.fetchUsers()
	}

	if cfg.Networks.Fetch.URL != "" {
		c.fetchNetworks()
	}

	if cfg.Users.Fetch.File != "" {
		c.readUsers()
	}

	if cfg.Networks.Fetch.File != "" {
		c.readNetworks()
	}

	for ip, id := range cfg.Users.Users {
		netIP := net.ParseIP(ip)
		if netIP == nil {
			log.Printf(fmt.Sprintf("Could not parse ip %s for user %s, skipping", ip, id))
			continue
		}

		intIP := IP2Int(netIP)
		c.Users[intIP] = id
	}

	for cidr, class := range cfg.Networks.Networks {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Println(fmt.Sprintf("Could not parse CIDR %s, class %s %v", cidr, class, err))
			continue
		}

		if class == LOCAL {
			c.Local = append(c.Local, *network)
		}

		if class == PEERING {
			c.Peering = append(c.Peering, *network)
		}
	}

	return c
}

func (c *Classifier) fetchUsers() {
	log.Println(fmt.Sprintf("Fetching users from url %s", c.Config.Users.Fetch.URL))

	resp, err := http.Get(c.Config.Users.Fetch.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	contentType := resp.Header.Get("Content-Type")

	log.Println(fmt.Sprintf("Users fetched statusCode:%d type:%s", resp.StatusCode, contentType))

	if strings.Contains(contentType, "application/json") {
		c.parseJSONUsers(string(body))
	}

	if strings.Contains(contentType, "text/csv") {
		c.parseCSVUsers(string(body))
	}
}

func (c *Classifier) readUsers() {
	log.Println(fmt.Sprintf("Reading users from file %s", c.Config.Users.Fetch.File))

	body, err := ioutil.ReadFile(c.Config.Users.Fetch.File)
	if err != nil {
		log.Fatal(err)
	}

	ext := filepath.Ext(c.Config.Users.Fetch.File)

	log.Println(fmt.Sprintf("Users readed extension:%s", ext))

	if ext == ".json" {
		c.parseJSONUsers(string(body))
	}

	if ext == ".csv" {
		c.parseCSVUsers(string(body))
	}
}

func (c *Classifier) fetchNetworks() {
	log.Println(fmt.Sprintf("Fetching networks from url %s", c.Config.Networks.Fetch.URL))

	resp, err := http.Get(c.Config.Networks.Fetch.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	contentType := resp.Header.Get("Content-Type")

	log.Println(fmt.Sprintf("Networks fetched statusCode:%d type:%s", resp.StatusCode, contentType))

	if strings.Contains(contentType, "application/json") {
		c.parseJSONNetworks(string(body))
	}

	if strings.Contains(contentType, "text/csv") {
		c.parseCSVNetworks(string(body))
	}
}

func (c *Classifier) readNetworks() {
	log.Println(fmt.Sprintf("Reading networks from file %s", c.Config.Networks.Fetch.File))

	body, err := ioutil.ReadFile(c.Config.Networks.Fetch.File)
	if err != nil {
		log.Fatal(err)
	}

	ext := filepath.Ext(c.Config.Networks.Fetch.File)

	log.Println(fmt.Sprintf("Networks readed extension:%s", ext))

	if ext == ".json" {
		c.parseJSONUsers(string(body))
	}

	if ext == ".csv" {
		c.parseCSVUsers(string(body))
	}
}

func (c *Classifier) parseJSONUsers(body string) {
	log.Println("Parsing JSON users")

	var result map[string]string
	err := json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Fatal(err)
	}

	parsed := 0
	for cidr, id := range result {
		c.Config.Users.Users[cidr] = id
		parsed = parsed + 1
	}

	log.Println(fmt.Sprintf("Parsed %d users ", parsed))
}

func (c *Classifier) parseCSVUsers(body string) {
	r := csv.NewReader(strings.NewReader(body))
	r.Comma = rune(c.Config.Users.Fetch.Comma[0])

	parsed := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		id := record[c.Config.Users.Fetch.IDField]
		cidr := record[c.Config.Users.Fetch.CIDRField]

		c.Config.Users.Users[cidr] = id
		parsed = parsed + 1
	}

	log.Println(fmt.Sprintf("Parsed %d users ", parsed))
}

func (c *Classifier) parseJSONNetworks(body string) {
	log.Println("Parsing JSON networks")

	var result map[string]string
	err := json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Fatal(err)
	}

	parsed := 0
	for cidr, class := range result {
		c.Config.Networks.Networks[cidr] = class
		parsed = parsed + 1
	}

	log.Println(fmt.Sprintf("Parsed %d networks", parsed))
}

func (c *Classifier) parseCSVNetworks(body string) {
	r := csv.NewReader(strings.NewReader(body))
	r.Comma = rune(c.Config.Networks.Fetch.Comma[0])

	parsed := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		cidr := record[c.Config.Networks.Fetch.CIDRField]
		class := record[c.Config.Networks.Fetch.ClassField]

		c.Config.Networks.Networks[cidr] = class
		parsed = parsed + 1
	}

	log.Println(fmt.Sprintf("Parsed %d networks", parsed))
}

//
// Classify entry
//
func (c *Classifier) Classify(entry *Entry) {

	class := UNKNOWN
	dir := UNKNOWN
	var clientIP, remoteIP *net.IP

	entry.Class = class
	entry.Dir = dir

	for _, localNet := range c.Local {
		if localNet.Contains(entry.SrcIP) {
			clientIP = &entry.SrcIP
			remoteIP = &entry.DstIP
			dir = OUT
			class = INTERNET
		}

		if localNet.Contains(entry.DstIP) {
			clientIP = &entry.DstIP
			remoteIP = &entry.SrcIP
			dir = IN
			class = INTERNET
		}

		if remoteIP != nil {
			if localNet.Contains(*remoteIP) {
				class = LOCAL
			}
		}
	}

	if remoteIP != nil {
		for _, peeringNet := range c.Peering {
			if peeringNet.Contains(*remoteIP) {
				class = PEERING
			}
		}

		if c.Multicast.Contains(*remoteIP) {
			class = MULTICAST
		}
	}

	if clientIP != nil {
		intIP := IP2Int(*clientIP)
		if id, ok := c.Users[intIP]; ok {
			entry.UserID = id
		}

		entry.Dir = dir
		entry.Class = class
	}
}

// IP2Int Convert net.IP to uint32
func IP2Int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
