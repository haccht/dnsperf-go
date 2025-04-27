# dnsperf-go

`dnsperf-go` is a tool written in Go to test performance of a DNS server.

## Usage:

```
Usage:
  dnsperf-go [OPTIONS]

Application Options:
  -d, --filepth=           The input data file (required)
  -s, --server=            DNS server address to query (default: 127.0.0.1:53)
  -m, --protocol=[udp|tcp] Set transport mode (default: udp)
  -t, --timeout=           The timeout for query completion (default: 1s)
  -l, --duration=          Run for at most this duration (default: 10s)
  -n, --max-sweep=         Run through input at most N times
  -c, --clients=           The number of concurrent clients (default: 1)
  -r, --shuffle            Shuffle input
  -Q, --rate-limit=        Limit the number of QPS (default: 1)
  -S, --show-qps=          The interval to show realtime QPS (default: 0s)
  -v, --detail             Print detail stats

Help Options:
  -h, --help               Show this help message

```

```
$ dnsperf-go -s 10.0.0.1:53 -d names.txt -l10s -Q200 -c300 -S1s
2025/04/27 11:42:24 Sending queries to 10.0.0.1:53
2025/04/27 11:42:24 Duration=10s  RateLimit=200q/s  Workers=300
2025/04/27 11:42:25     Rate=199.0q/s   Sent=199        Loss=0  NOERROR=199
2025/04/27 11:42:26     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:27     Rate=199.0q/s   Sent=199        Loss=0  NOERROR=199
2025/04/27 11:42:28     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:29     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:30     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:31     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:32     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:33     Rate=200.0q/s   Sent=200        Loss=0  NOERROR=200
2025/04/27 11:42:34 Performance test completed

Statistics
  Queries sent:               2003 reqs
  Queries completed:          2003 reqs 100.0%
  Queries lost:                  0 reqs   0.0%
  Queries per seconds:       200.1 q/s
  Run time:                  10.01 sec
  Latency(min):                  0 msec
  Latency(avg):                  7 msec
  Latency(max):                134 msec
  Latency(stddev):               5 msec
  Request size(avg):            43 bytes
  Response size(avg):           84 bytes

Statistics per Rcode
   NOERROR count:             2003 reqs
```

## Input file format

```
example.com A
example.com AAAA
www.example.jp A
www.example.jp AAAA
```
