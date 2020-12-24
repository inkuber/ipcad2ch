# IPCAD to ClickHouse saver

Small UNIX way program to write network statistics into wonderful column based database ClickHouse.
It is useful for making network accounting programs. `ipcad2ch` reads ipcad output from stdin, parse it, classify by users, network classes and direction and writes data to clickhouse.
Database contains main `details` table that stores all information as is, also there are three aggregations materialized views for daily, hourly and minutely statistics. If no tables in database, utility creates them itself.

## Configuration

Utility by default looks file `ipcad2ch.yaml` in `/etc/`, `/usr/local/etc/` and current dir. Also you could point custom config file with `-f <file>` flag.

```yaml
ipcad:
    # Print input data to stdout for following processing
    # Example:
    #   $RSH -l root $IP clear ip accounting > /dev/null
    #   $RSH -l root $IP show ip accounting checkpoint | ipcad2ch > $FILE 2>/tmp/last_ipcad2ch
    pipe: true

clickhouse:
    host: 'clickhouse'
    # user: user
    # password: password
    # database: database
    port: 9000

    # number of records saving in one insert query
    bunch: 1000000

classifier:
    users:
        # Fetch users from url, allowed formats: csv json
        #
        fetch:
          # url: http://nginx/users.csv
          file: examples/classifier/users.csv

          # Field separator for CSV
          # Comma: ";"

          # Field positions for CSV
          # IDField: 0
          # CIDRField: 1

        # Users could be set manually
        users:
           "1": "188.218.189.188/32"

    networks:
        # Fetch networks from url, allowed formats: csv json
        fetch:
          url: http://nginx/networks.csv

          # Field separator for CSV
          # Comma: ";"

          # Field positions for CSV
          # cidrField: 0
          #
          # Could be "local" or "peering"
          # classField: 1

        # Networks could be set manually
        networks:
           "188.218.0.0/16": "local"
           "103.203.0.0/16": "peering"
```

# Database

## Details table

```sql
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
```

# Daily table

```sql
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
```

# Hourly table

```sql
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
```

# Minutely table

```sql
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
```

# Dictionaries

## Users information

Users information could be set with 3 ways:

* Fetching data from http (json, csv formats)
* Reading data from file (json, csv formats)
* Setting in configuration yaml file

### Formats

* JSON format must be map of IP(string) => ID(string)
```json
{
    "192.168.0.1": "1"
}
```

* CSV standart format, by default: `"IP", "ID"`, but could be changed in config

## Network information

There are four network classes:

* Local - traffic between hosts of the provider networks
* Peering - traffic to peering partners, could be friendly ISP
* Internet - all other network traffic in the world
* Multicast - special traffic for analytics

Networks could be set with same 3 ways as users:

* Fetching data from http (json, csv formats)
* Reading data from file (json, csv formats)
* Setting in configuration yaml file

### Formats
* JSON format must be map of CIDR(string) => Class(string)
```json
{
    "192.168.0.0/16": "local",
    "10.10.0.0/16": "peering"
}
```

* CSV standart format, by default: `"CIDR", "Class"`, but could be changed in config
