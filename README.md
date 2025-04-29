# dnsperf-go

`dnsperf-go` is a tool written in Go to test performance of a DNS server.

## Usage:

```
Usage:
  dnsperf-go [OPTIONS]

Application Options:
  -d, --input=              Path to query list file (required)
  -s, --server=             DNS server address (default: 127.0.0.1:53)
  -m, --transport=[udp|tcp] Network transport mode (default: udp)
  -t, --timeout=            Timeout for query completion (default: 1s)
  -l, --duration=           Total benchmark duration (default: 10s)
  -n, --loops=              Maximum passes over the input list (0 = unlimited) (default: 0)
  -c, --workers=            Number of concurrent workers (default: 1)
  -Q, --qps=                Global query-per-seconds limit (default: 1)
  -S, --realtime-stats=     Print stats every N seconds (0s = disable) (default: 0s)
  -p, --per-query-stats     Print stats per queries (default: false)
  -r, --shuffle             Shuffle input (default: false)

Help Options:
  -h, --help                Show this help message
```

```
$ dnsperf-go -s 127.0.0.1:53 -d r0names.txt -Q 2000 -c 4000 -l 5s -S 1s
2025/04/30 08:21:32 Sending queries to 127.0.0.1:53/udp
2025/04/30 08:21:32 Duration=5s  RateLimit=2000q/s  Workers=4000
2025/04/30 08:21:33     QPS=1685.0q/s   Sent=1685       Loss=0          NOERROR=1685
2025/04/30 08:21:34     QPS=1726.0q/s   Sent=1726       Loss=0          NOERROR=1726
2025/04/30 08:21:35     QPS=1766.0q/s   Sent=1766       Loss=0          NOERROR=1766
2025/04/30 08:21:36     QPS=1754.0q/s   Sent=1754       Loss=0          NOERROR=1754
2025/04/30 08:21:37 Performance test completed

Statistics
  Queries sent:               8691 reqs
  Queries completed:          8691 reqs 100.0%
  Queries lost:                  0 reqs   0.0%
  Queries per seconds:      1734.7 q/s
  Run time:                    5.0 sec
  Latency(min):               0.30 msec
  Latency(avg):               7.83 msec
  Latency(max):              12.94 msec
  Latency(stddev):            2.30 msec
  Request size(avg):          43.0 bytes
  Response size(avg):         84.0 bytes

Statistics per Rcode
  NOERROR count:      8691 reqs
```

## Input file format

```
example.com A
example.com AAAA
www.example.jp A
www.example.jp AAAA
```
