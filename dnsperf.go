package main

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/miekg/dns"
)

type DNSPerf struct {
	mu      sync.Mutex
	stat    []*DNSPerfResult
	sent    int
	lost    int
	success int

	Server string
	Client *dns.Client
}

func NewDNSPerf(server, protocol string, timeout time.Duration) *DNSPerf {
	return &DNSPerf{
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
	p.sent++
	if res.err != nil || res.m == nil {
		p.lost++
	} else if res.m.Rcode == dns.RcodeSuccess {
		p.success++
	}
	p.stat = append(p.stat, res)
	p.mu.Unlock()
}

func (p *DNSPerf) Sent() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.sent
}

func (p *DNSPerf) Lost() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.lost
}

func (p *DNSPerf) PrintStats(cfg *Config) {
	p.mu.Lock()
	stat := p.stat
	sent := p.sent
	lost := p.lost
	success := p.success
	p.mu.Unlock()

	var reqSize, respSize int
	var minRTT, maxRTT, sumRTT, sqsumRTT int64
	rcodeCount := make(map[int]int)
	for _, res := range stat {
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
			rcodeCount[res.m.Rcode]++
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	lostRate := 100 * (float64(lost) / float64(sent))

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent: \t%10d reqs\n", sent)
	fmt.Fprintf(w, "  Queries completed: \t%10d reqs\t%5.1f%%\n", sent-lost, 100-lostRate)
	fmt.Fprintf(w, "  Queries lost: \t%10d reqs\t%5.1f%%\n", lost, lostRate)
	fmt.Fprintf(w, "  Queries per seconds: \t%10.1f q/s\n", float64(sent)/cfg.Duration.Seconds())
	fmt.Fprintf(w, "  Request size(avg): \t%10d bytes\n", reqSize/sent)
	fmt.Fprintf(w, "  Response size(avg): \t%10d bytes\n", respSize/sent)
	if success > 0 {
		avgRTT := sumRTT / int64(sent)
		stddevRTT := math.Sqrt(float64(sqsumRTT)/float64(sent) - float64(avgRTT*avgRTT))

		fmt.Fprintf(w, "  Latency(min): \t%10d ms\n", minRTT)
		fmt.Fprintf(w, "  Latency(avg): \t%10d ms\n", avgRTT)
		fmt.Fprintf(w, "  Latency(max): \t%10d ms\n", maxRTT)
		fmt.Fprintf(w, "  Latency(stddev): \t%10d ms\n", int(stddevRTT))

		fmt.Fprintln(w, "\nStatistics per Rcode")
		for rcode, rcodeStr := range dns.RcodeToString {
			cnt := rcodeCount[rcode]
			if cnt > 0 {
				fmt.Fprintf(w, "  %8s count: \t%10d reqs\n", rcodeStr, cnt)
			}
		}
	}

	if cfg.ShowDetail {
		resMap := make(map[DNSPerfRequest][]*DNSPerfResult)
		for _, s := range stat {
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

		fmt.Fprintln(w, "\nStatistics per query")
		for _, req := range reqs {
			var lost int
			var sumRTT int64

			for _, res := range resMap[req] {
				if res.err != nil || res.m == nil {
					lost++
				}
				rtt := res.rtt.Milliseconds()
				sumRTT += rtt
			}

			l := len(resMap[req])
			r := 100 * float64(l-lost) / float64(l)
			fmt.Fprintf(w, "  %s\t%10d reqs\t%5.1f%% OK\tRTT: %d ms\n", req.Question(), l, r, sumRTT/int64(l))
		}
	}

	w.Flush()
}
