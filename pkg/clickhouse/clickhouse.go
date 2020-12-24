package clickhouse

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/ClickHouse/clickhouse-go"
	"github.com/inkuber/ipcad2ch/pkg/classifier"
	"github.com/inkuber/ipcad2ch/pkg/ipcad"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	User      string `mapstructure:"user"`
	Password  string `mapstructure:"password"`
	Database  string `mapstructure:"database"`
	BunchSize int    `mapstructure:"bunch"`
}

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

func Write(wg *sync.WaitGroup, cfg Config, c classifier.Classifier, in chan *ipcad.Entry) {
	log.Println("Starting clickhouse write coroutine")

	defer wg.Done()

	db, err := connect(cfg)
	defer db.Close()

	if err != nil {
		log.Fatal(err)
	}

	err = initTables(db)
	if err != nil {
		log.Fatal(err)
	}

	bunch := make([]Entry, cfg.BunchSize)

	var chWg sync.WaitGroup

	chWg.Add(1)
	go func() {
		index := 0
		for {
			select {
			case e := <-in:
				if e == nil {
					if index > 0 {
						tosave := bunch[:index]
						err := save(db, tosave)
						if err != nil {
							log.Fatal(err)
						}
					}

					chWg.Done()
					log.Println("Clickhouse read coroutine ended")
					return
				}

				ipcadEntry := *e

				if index < cfg.BunchSize {
					if ipcadEntry.SrcIP == nil || ipcadEntry.DstIP == nil {
						log.Fatal("nil src or dst passed")
					}

					entry := Entry{
						SrcIP:     ipcadEntry.SrcIP,
						DstIP:     ipcadEntry.DstIP,
						Packets:   ipcadEntry.Packets,
						Bytes:     ipcadEntry.Bytes,
						SrcPort:   ipcadEntry.SrcPort,
						DstPort:   ipcadEntry.DstPort,
						Proto:     ipcadEntry.Proto,
						Iface:     ipcadEntry.Iface,
						Collected: ipcadEntry.Collected,
					}

					e := classifier.Entry(entry)
					c.Classify(&e)

					bunch[index] = Entry(e)

					index = index + 1
				} else {
					err := save(db, bunch)
					if err != nil {
						log.Fatal(err)
					}
					index = 0
				}
			}
		}
	}()

	chWg.Wait()
}

func save(db *sql.DB, bunch []Entry) error {
	log.Println(fmt.Sprintf("Saving bunch of records to clickhouse [len=%d cap=%d]", len(bunch), cap(bunch)))

	insertQuery := `
		INSERT INTO details (
			collected,
			user_id,
			dir,
			class,
			src_ip,
			src_port,
			dst_ip,
			dst_port,
			packets,
			bytes,
			proto
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range bunch {
		if e.SrcIP == nil || e.DstIP == nil {
			log.Fatal(fmt.Sprintf("Nil passed in SrcIP or DstIP"))
		}

		srcIP := ip2int(e.SrcIP)
		dstIP := ip2int(e.DstIP)

		_, err := stmt.Exec(
			e.Collected,
			e.UserID,
			e.Dir,
			e.Class,
			srcIP,
			e.SrcPort,
			dstIP,
			e.DstPort,
			e.Packets,
			e.Bytes,
			e.Proto,
		)

		if err != nil {
			log.Fatal(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func connect(cfg Config) (*sql.DB, error) {
	log.Println("Connecting to clickhouse")

	var params []string
	if cfg.User != "" {
		params = append(params, fmt.Sprintf("username=%s", cfg.User))
	}

	if cfg.Password != "" {
		params = append(params, fmt.Sprintf("password=%s", cfg.Password))
	}

	if cfg.Database != "" {
		params = append(params, fmt.Sprintf("database=%s", cfg.Database))
	}

	DSN := fmt.Sprintf("tcp://%s:%d?%s", cfg.Host, cfg.Port, strings.Join(params, "&"))
	log.Println(fmt.Sprintf("DSN: %s", DSN))

	db, err := sql.Open("clickhouse", DSN)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Println(fmt.Sprintf("[%d] %s", exception.Code, exception.Message))
			log.Println(exception.StackTrace)
		} else {
			return nil, err
		}
	}

	return db, nil
}

func initTables(db *sql.DB) error {
	log.Println("Checking tables exists in clickhouse")

	detailsQuery := `
		CREATE TABLE IF NOT EXISTS details
		(
			collected DateTime,
			user_id String,
			dir Enum8('unknown' = 0, 'in' = 1, 'out' = 2),
			class Enum8('unknown' = 0, 'local' = 1, 'peering' = 2, 'internet' = 3, 'multicast' = 4),
			src_ip UInt32,
			src_port UInt16,
			dst_ip UInt32,
			dst_port UInt16,
			packets UInt16,
			bytes UInt32,
			proto UInt8
		)
		ENGINE = MergeTree
		PARTITION BY toYYYYMMDD(collected)
		ORDER BY (collected, user_id, dir, class, src_ip, dst_ip, proto)
		SETTINGS index_granularity = 8192
	`

	dailyQuery := `
		CREATE MATERIALIZED VIEW IF NOT EXISTS daily
		(
			date Date,
			user_id String,
			class Enum8('unknown' = 0, 'local' = 1, 'peering' = 2, 'internet' = 3, 'multicast' = 4),
			dir Enum8('unknown' = 0, 'in' = 1, 'out' = 2),
			bytes AggregateFunction(sum, UInt32)
		)
		ENGINE = AggregatingMergeTree()
		PARTITION BY toYYYYMM(date)
		ORDER BY (date, user_id, class, dir)
		SETTINGS index_granularity = 8192 AS
		SELECT
			toDate(collected) AS date,
			user_id,
			class,
			dir,
			sumState(bytes) AS bytes
		FROM details
		GROUP BY
			toDate(collected),
			user_id,
			class,
			dir
	`

	hourlyQuery := `
		CREATE MATERIALIZED VIEW IF NOT EXISTS hourly
		(
			date DateTime,
			user_id String,
			class Enum8('unknown' = 0, 'local' = 1, 'peering' = 2, 'internet' = 3, 'multicast' = 4),
			dir Enum8('unknown' = 0, 'in' = 1, 'out' = 2),
			bytes AggregateFunction(sum, UInt32)
		)
		ENGINE = AggregatingMergeTree()
		PARTITION BY toYYYYMM(date)
		ORDER BY (date, user_id, class, dir)
		SETTINGS index_granularity = 8192 AS
		SELECT
			toStartOfHour(collected) AS date,
			user_id,
			class,
			dir,
			sumState(bytes) AS bytes
		FROM details
		GROUP BY
			toStartOfHour(collected),
			user_id,
			class,
			dir
	`

	minutelyQuery := `
		CREATE MATERIALIZED VIEW IF NOT EXISTS minutely
		(
			date DateTime,
			user_id String,
			class Enum8('unknown' = 0, 'local' = 1, 'peering' = 2, 'internet' = 3, 'multicast' = 4),
			dir Enum8('unknown' = 0, 'in' = 1, 'out' = 2),
			bytes AggregateFunction(sum, UInt32)
		)
		ENGINE = AggregatingMergeTree()
		PARTITION BY toYYYYMM(date)
		ORDER BY (date, user_id, class, dir)
		SETTINGS index_granularity = 8192 AS
		SELECT
			toStartOfMinute(collected) AS date,
			user_id,
			class,
			dir,
			sumState(bytes) AS bytes
		FROM details
		GROUP BY
			toStartOfMinute(collected),
			user_id,
			class,
			dir
	`

	_, err := db.Exec(detailsQuery)
	if err != nil {
		return err
	}

	_, err = db.Exec(dailyQuery)
	if err != nil {
		return err
	}

	_, err = db.Exec(hourlyQuery)
	if err != nil {
		return err
	}

	_, err = db.Exec(minutelyQuery)
	if err != nil {
		return err
	}

	return nil
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
