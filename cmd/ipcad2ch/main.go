package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/inkuber/ipcad2ch/pkg/classifier"
	"github.com/inkuber/ipcad2ch/pkg/clickhouse"
	"github.com/inkuber/ipcad2ch/pkg/ipcad"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"os"
	"sync"
	"time"
)

type Config struct {
	Config string `mapstructure:"config"`
	File   string `mapstructure:"file"`
	Buffer int    `mapstructure:"buffer"`

	Ipcad      ipcad.Config
	Clickhouse clickhouse.Config
	Classifier classifier.Config
}

func ParseConfig() Config {
	log.Println("IPCAD to ClickHouse")

	cfg := Config{}

	v := viper.NewWithOptions(viper.KeyDelimiter("::"))

	flag.String("config", "", "Config file")
	flag.String("file", "stdin", "Read IPCAD from file")
	flag.String("ipcad.collected", "", "Collected time")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)

	config := v.GetString("config")

	if config == "" {
		v.SetConfigName("ipcad2ch")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/")
		v.AddConfigPath("/usr/local/etc/")
		v.AddConfigPath(".")
	} else {
		log.Println(fmt.Sprintf("Reading config from file %s", config))
		v.SetConfigFile(config)
	}

	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	v.SetDefault("Clickhouse::Bunch", 100000)
	v.SetDefault("Buffer", 100)

	t := time.Now()
	v.SetDefault("Ipcad::Collected", t.Format(time.RFC3339))

	v.SetDefault("Classifier::Users::Fetch::Comma", ";")
	v.SetDefault("Classifier::Users::Fetch::IDField", 0)
	v.SetDefault("Classifier::Users::Fetch::CIDRField", 1)

	v.SetDefault("Classifier::Networks::Fetch::Comma", ";")
	v.SetDefault("Classifier::Networks::Fetch::CIDRField", 0)
	v.SetDefault("Classifier::Networks::Fetch::ClassField", 1)

	v.Unmarshal(&cfg)

	b, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		log.Fatal(err)
		return cfg
	}

	log.Println("Parameters:")
	log.Println(string(b))

	return cfg
}

func main() {

	cfg := ParseConfig()

	in := os.Stdin
	if cfg.File != "stdin" {
		f, err := os.Open(cfg.File)
		if err != nil {
			log.Fatal(err)
		}
		in = f
	}

	var wg sync.WaitGroup

	classifier := classifier.NewClassifier(cfg.Classifier)

	entries := make(chan *ipcad.Entry, cfg.Buffer)
	log.Println(fmt.Sprintf("entries [len=%d cap=%d]", len(entries), cap(entries)))

	wg.Add(1)
	go ipcad.Read(&wg, cfg.Ipcad, in, entries)

	wg.Add(1)
	go clickhouse.Write(&wg, cfg.Clickhouse, classifier, entries)

	wg.Wait()
}
