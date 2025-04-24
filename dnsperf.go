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
	prev    *DNSPerfStat
	stat    *DNSPerfStat
	results []*DNSPerfResult

	Server string
	Client *dns.Client
}

func NewDNSPerf(server, protocol string, timeout time.Duration) *DNSPerf {
	return &DNSPerf{
		prev:    &DNSPerfStat{rcodes: make(map[int]int)},
		stat:    &DNSPerfStat{rcodes: make(map[int]int)},
		results: make([]*DNSPerfResult, 0),

		Server: server,
		Client: &dns.Client{
			Net:     protocol,
			Timeout: timeout,
		},
	}
}

type DNSPerfRequest struct {
	m *dns.Msg
}

func (r *DNSPerfRequest) Question() string {
	q := r.m.Question[0]
	return fmt.Sprintf("%s %s", q.Name, dns.TypeToString[q.Qtype])
}

type DNSPerfResult struct {
	m   *dns.Msg
	t   time.Time
	rtt time.Duration
	req *DNSPerfRequest
	err error
}

func (p *DNSPerf) Perform(req *DNSPerfRequest) {
	start := time.Now()
	m, _, err := p.Client.Exchange(req.m, p.Server)

	p.mu.Lock()
	res := &DNSPerfResult{
		m:   m,
		t:   start,
		rtt: time.Since(start),
		req: req,
		err: err,
	}
	p.stat.sent++
	if res.err != nil || res.m == nil {
		p.stat.lost++
	} else if res.m != nil {
        p.stat.rcodes[res.m.Rcode]++
        if res.m.Rcode == dns.RcodeSuccess {
            p.stat.success++
        }
    }
	p.results = append(p.results, res)
	p.mu.Unlock()
}

func (p *DNSPerf) Tick(cfg *Config) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf bytes.Buffer
	qps := float64(p.stat.sent-p.prev.sent) / cfg.QPSInterval.Seconds()

	fmt.Fprintf(&buf, "\tRate=%.1fq/s\tSent=%d\tLoss=%d", qps, p.stat.sent-p.prev.sent, p.stat.lost-p.prev.lost)
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

func (p *DNSPerf) Statistics(cfg *Config) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf bytes.Buffer
	var reqSize, respSize int
	var minRTT, maxRTT, sumRTT, sqsumRTT int64
	for _, res := range p.results {
		rtt := res.rtt.Milliseconds()

		sumRTT += rtt
		sqsumRTT += rtt * rtt

		if minRTT > rtt || minRTT == 0 {
			minRTT = rtt
		}

		if maxRTT < rtt || maxRTT == 0 {
			maxRTT = rtt
		}

		if res.err == nil && res.m != nil {
			reqSize += res.req.m.Len()
			respSize += res.m.Len()
		}
	}

	w := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)
	lostRate := 100 * (float64(p.stat.lost) / float64(p.stat.sent))

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent: \t%10d reqs\n", p.stat.sent)
	fmt.Fprintf(w, "  Queries completed: \t%10d reqs\t%5.1f%%\n", p.stat.sent-p.stat.lost, 100-lostRate)
	fmt.Fprintf(w, "  Queries lost: \t%10d reqs\t%5.1f%%\n", p.stat.lost, lostRate)
	fmt.Fprintf(w, "  Queries per seconds: \t%10.1f q/s\n", float64(p.stat.sent)/cfg.Duration.Seconds())
	fmt.Fprintf(w, "  Request size(avg): \t%10d bytes\n", reqSize/p.stat.sent)
	fmt.Fprintf(w, "  Response size(avg): \t%10d bytes\n", respSize/p.stat.sent)
	if p.stat.lost == 0 {
		avgRTT := sumRTT / int64(p.stat.sent)
		stddevRTT := math.Sqrt(float64(sqsumRTT)/float64(p.stat.sent) - float64(avgRTT*avgRTT))

		fmt.Fprintf(w, "  Latency(min): \t%10d ms\n", minRTT)
		fmt.Fprintf(w, "  Latency(avg): \t%10d ms\n", avgRTT)
		fmt.Fprintf(w, "  Latency(max): \t%10d ms\n", maxRTT)
		fmt.Fprintf(w, "  Latency(stddev): \t%10d ms\n", int(stddevRTT))

		fmt.Fprintln(w, "\nStatistics per Rcode")
		for rcode := range len(dns.RcodeToString) {
			cnt := p.stat.rcodes[rcode]
			if cnt > 0 {
				fmt.Fprintf(w, "  %8s count: \t%10d reqs\n", dns.RcodeToString[rcode], cnt)
			}
		}
	}

	w.Flush()

	if cfg.ShowDetail {
		resMap := make(map[DNSPerfRequest][]*DNSPerfResult)
		for _, s := range p.results {
			if len(resMap[*s.req]) == 0 {
				resMap[*s.req] = []*DNSPerfResult{}
			}

			resMap[*s.req] = append(resMap[*s.req], s)
		}

		reqs := make([]DNSPerfRequest, 0, len(resMap))
		for q := range resMap {
			reqs = append(reqs, q)
		}
		slices.SortFunc(reqs, func(x, y DNSPerfRequest) int {
			kx := x.Question()
			ky := y.Question()
			return cmp.Compare(kx, ky)
		})

		fmt.Fprintln(&buf, "\nStatistics per query")
		for _, req := range reqs {
			var lost int
			var sumRTT int64

            rcodes := make(map[int]int)
			for _, res := range resMap[req] {
				if res.err != nil || res.m == nil {
					lost++
				}
                rcodes[res.m.Rcode]++
				rtt := res.rtt.Milliseconds()
				sumRTT += rtt
			}

			s := len(resMap[req])
			r := 100 * float64(lost) / float64(s)
            fmt.Fprintf(&buf, "  %s   =>\tTotal: %d", req.Question(), s)
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
