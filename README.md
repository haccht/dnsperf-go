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
2025/04/12 12:30:04 Sending queries to 10.0.0.1:53
2025/04/12 12:30:04 Duration=10s  RateLimit=200q/s  Workers=300
2025/04/12 12:30:05     Sent: 99 reqs           Loss: 0 reqs         QPS: 99.0 q/s
2025/04/12 12:30:06     Sent: 300 reqs          Loss: 0 reqs         QPS: 201.0 q/s
2025/04/12 12:30:07     Sent: 499 reqs          Loss: 0 reqs         QPS: 199.0 q/s
2025/04/12 12:30:08     Sent: 699 reqs          Loss: 0 reqs         QPS: 200.0 q/s
2025/04/12 12:30:09     Sent: 899 reqs          Loss: 0 reqs         QPS: 200.0 q/s
2025/04/12 12:30:10     Sent: 1098 reqs         Loss: 0 reqs         QPS: 199.0 q/s
2025/04/12 12:30:11     Sent: 1298 reqs         Loss: 0 reqs         QPS: 200.0 q/s
2025/04/12 12:30:12     Sent: 1498 reqs         Loss: 0 reqs         QPS: 200.0 q/s
2025/04/12 12:30:13     Sent: 1699 reqs         Loss: 0 reqs         QPS: 201.0 q/s
2025/04/12 12:30:14     Sent: 1898 reqs         Loss: 0 reqs         QPS: 199.0 q/s
2025/04/12 12:30:17 Performance test completed

Statistics
  Queries sent:               2065 reqs
  Queries completed:          2065 reqs 100.0%
  Queries lost:                  0 reqs   0.0%
  Queries per seconds:       206.5 q/s
  Request size(avg):            31 bytes
  Response size(avg):          162 bytes
  Latency(min):                  7 ms
  Latency(avg):                 12 ms
  Latency(max):                122 ms
  Latency(stddev):              17 ms

Statistics per Rcode
   NOERROR count:             1861 reqs
  NXDOMAIN count:              204 reqs
```

## Input file format

```
example.com A
example.com AAAA
www.example.jp A
www.example.jp AAAA
```
