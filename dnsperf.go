package main

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/miekg/dns"
)

type DNSPerfStat struct {
	sent    int
	lost    int
	success int
	rcodes  map[int]int
}

type DNSPerf struct {
	mu      sync.Mutex
	stat    *DNSPerfStat
	prev    *DNSPerfStat
	results []*DNSPerfResult

	Server string
	Client *dns.Client
}

func NewDNSPerf(server, protocol string, timeout time.Duration) *DNSPerf {
	return &DNSPerf{
		stat:    &DNSPerfStat{rcodes: make(map[int]int)},
		prev:    &DNSPerfStat{rcodes: make(map[int]int)},
		results: []*DNSPerfResult{},

		Server: server,
		Client: &dns.Client{
			Net:     protocol,
			Timeout: timeout,
		},
	}
}

type DNSPerfRequest struct {
	m   *dns.Msg
	key string
}

type DNSPerfResult struct {
	m   *dns.Msg
	t   time.Time
	rtt time.Duration
	req *DNSPerfRequest
}

func (p *DNSPerf) Perform(req *DNSPerfRequest) {
	start := time.Now()
	m, _, _ := p.Client.Exchange(req.m.Copy(), p.Server)

	p.mu.Lock()
	res := &DNSPerfResult{
		m:   m,
		t:   start,
		rtt: time.Since(start),
		req: req,
	}
	if res.m == nil {
		p.stat.lost++
	} else if res.m != nil {
		p.stat.rcodes[res.m.Rcode]++
		if res.m.Rcode == dns.RcodeSuccess {
			p.stat.success++
		}
	}
	p.stat.sent++
	p.results = append(p.results, res)
	p.mu.Unlock()
}

func (p *DNSPerf) Tick(interval time.Duration) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf bytes.Buffer
	qps := float64(p.stat.sent-p.prev.sent) / interval.Seconds()

	fmt.Fprintf(&buf, "\tRate=%.1fq/s \tSent=%d \tLoss=%d", qps, p.stat.sent-p.prev.sent, p.stat.lost-p.prev.lost)
	for rcode := range len(dns.RcodeToString) {
		cnt := p.stat.rcodes[rcode] - p.prev.rcodes[rcode]
		if cnt > 0 {
			fmt.Fprintf(&buf, "\t%s=%d", dns.RcodeToString[rcode], cnt)
		}

		p.prev.rcodes[rcode] = p.stat.rcodes[rcode]
	}

	p.prev.sent = p.stat.sent
	p.prev.lost = p.stat.lost
	p.prev.success = p.stat.success
	return buf.String()
}

func (p *DNSPerf) Stat(duration time.Duration, verbose bool) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf bytes.Buffer

	w := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)
	received := p.stat.sent - p.stat.lost
	lostRate := 100 * (float64(p.stat.lost) / float64(p.stat.sent))

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent: \t%10d reqs\n", p.stat.sent)
	fmt.Fprintf(w, "  Queries completed: \t%10d reqs\t%5.1f%%\n", received, 100-lostRate)
	fmt.Fprintf(w, "  Queries lost: \t%10d reqs\t%5.1f%%\n", p.stat.lost, lostRate)
	fmt.Fprintf(w, "  Queries per seconds: \t%10.1f q/s\n", float64(p.stat.sent)/duration.Seconds())
	fmt.Fprintf(w, "  Run time: \t%10.2f sec\n", duration.Seconds())
	if p.stat.sent == 0 {
		w.Flush()
		return buf.String()
	}

	var reqSize, respSize int
	var minRTT, maxRTT, sumRTT, sqsumRTT int64
	for i, res := range p.results {
		rtt := res.rtt.Milliseconds()

		sumRTT += rtt
		sqsumRTT += rtt * rtt

		if minRTT > rtt || i == 0 {
			minRTT = rtt
		}

		if maxRTT < rtt || i == 0 {
			maxRTT = rtt
		}

		if res.m != nil {
			reqSize += res.req.m.Len()
			respSize += res.m.Len()
		}
	}

	avgRTT := sumRTT / int64(p.stat.sent)
	stddevRTT := math.Sqrt(float64(sqsumRTT)/float64(p.stat.sent) - float64(avgRTT*avgRTT))
	fmt.Fprintf(w, "  Latency(min): \t%10d msec\n", minRTT)
	fmt.Fprintf(w, "  Latency(avg): \t%10d msec\n", avgRTT)
	fmt.Fprintf(w, "  Latency(max): \t%10d msec\n", maxRTT)
	fmt.Fprintf(w, "  Latency(stddev): \t%10d msec\n", int(stddevRTT))
	if received != 0 {
		fmt.Fprintf(w, "  Request size(avg): \t%10d bytes\n", reqSize/received)
		fmt.Fprintf(w, "  Response size(avg): \t%10d bytes\n", respSize/received)

		fmt.Fprintln(w, "\nStatistics per Rcode")
		for rcode := range len(dns.RcodeToString) {
			cnt := p.stat.rcodes[rcode]
			if cnt > 0 {
				fmt.Fprintf(w, "  %8s count: \t%10d reqs\n", dns.RcodeToString[rcode], cnt)
			}
		}
	}
	w.Flush()

	if verbose {
		resMap := make(map[DNSPerfRequest][]*DNSPerfResult)
		for _, s := range p.results {
			resMap[*s.req] = append(resMap[*s.req], s)
		}

		reqs := make([]DNSPerfRequest, 0, len(resMap))
		for q := range resMap {
			reqs = append(reqs, q)
		}
		slices.SortFunc(reqs, func(x, y DNSPerfRequest) int {
			return cmp.Compare(x.key, y.key)
		})

		fmt.Fprintln(&buf, "\nStatistics per query")
		for _, req := range reqs {
			var lost int
			var sumRTT int64

			rcodes := make(map[int]int)
			for _, res := range resMap[req] {
				if res.m == nil {
					lost++
				}
				rcodes[res.m.Rcode]++
				rtt := res.rtt.Milliseconds()
				sumRTT += rtt
			}

			s := len(resMap[req])
			r := 100 * float64(lost) / float64(s)
			fmt.Fprintf(&buf, "  %s   =>\tTotal: %d", req.key, s)
			for rcode := range len(dns.RcodeToString) {
				cnt := rcodes[rcode]
				if cnt > 0 {
					fmt.Fprintf(&buf, "\t%s: %d", dns.RcodeToString[rcode], cnt)
				}
			}
			fmt.Fprintf(&buf, "\tLoss: %5.1f%%\tRTT: %d ms\n", r, sumRTT/int64(s))
		}
	}

	return buf.String()
}
