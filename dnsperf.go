package main

import (
	"fmt"
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

func (r *DNSPerfRequest) ToString() string {
	q := r.m.Question[0]
	return fmt.Sprintf("%s %s", q.Name, dns.TypeToString[q.Qtype])
}

type DNSPerfResult struct {
	q   *DNSPerfRequest
	m   *dns.Msg
	t   time.Time
	err error
	rtt time.Duration
}

func (p *DNSPerf) Query(q *DNSPerfRequest) {
	start := time.Now()
	m, _, err := p.Client.Exchange(q.m, p.Server)
	s := &DNSPerfResult{q: q, m: m, t: start, err: err, rtt: time.Since(start)}

	p.mu.Lock()
	p.sent++
	if s.err != nil || s.m == nil {
		p.lost++
	} else if s.m.Rcode == dns.RcodeSuccess {
		p.success++
	}
	p.stat = append(p.stat, s)
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

	var sumRTT, minRTT, maxRTT int64
	var reqSize, respSize int
	rcodeCount := make(map[string]int)
	for _, s := range stat {
		rtt := s.rtt.Milliseconds()
		sumRTT += rtt

		if minRTT > rtt || minRTT == 0 {
			minRTT = rtt
		}

		if maxRTT < rtt || maxRTT == 0 {
			maxRTT = rtt
		}

		if s.err == nil && s.m != nil {
			reqSize += s.q.m.Len()
			respSize += s.m.Len()

			rcode := dns.RcodeToString[s.m.Rcode]
			rcodeCount[rcode]++
		}
	}

	lostRate := 100 * (float64(lost) / float64(sent))

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent: \t%10d reqs\n", sent)
	fmt.Fprintf(w, "  Queries completed: \t%10d reqs\t%5.1f%%\n", sent-lost, 100-lostRate)
	fmt.Fprintf(w, "  Queries lost: \t%10d reqs\t%5.1f%%\n", lost, lostRate)
	fmt.Fprintf(w, "  Queries per seconds: \t%10.1f q/s\n", float64(sent)/cfg.Duration.Seconds())
	fmt.Fprintf(w, "  Request size(avg): \t%10d bytes\n", reqSize/sent)
	fmt.Fprintf(w, "  Response size(avg): \t%10d bytes\n", respSize/sent)
	if success > 0 {
		avgRTT := sumRTT / int64(success)
		fmt.Fprintf(w, "  Latency(min): \t%10d ms\n", minRTT)
		fmt.Fprintf(w, "  Latency(avg): \t%10d ms\n", avgRTT)
		fmt.Fprintf(w, "  Latency(max): \t%10d ms\n", maxRTT)
	}

	fmt.Fprintln(w, "\nStatistics per Rcode")
	for rcode, count := range rcodeCount {
		fmt.Fprintf(w, "  %8s count: \t%10d reqs\n", rcode, count)
	}

	if cfg.ShowDetail {
		spr := make(map[DNSPerfRequest][]*DNSPerfResult)
		for _, s := range stat {
			if len(spr[*s.q]) == 0 {
				spr[*s.q] = []*DNSPerfResult{}
			}

			spr[*s.q] = append(spr[*s.q], s)
		}

		keys := make([]DNSPerfRequest, 0, len(spr))
		for q := range spr {
			keys = append(keys, q)
		}
		slices.SortFunc(keys, func(x, y DNSPerfRequest) int {
			kx := x.ToString()
			ky := y.ToString()

			switch {
			case kx > ky:
				return 1
			case kx < ky:
				return -1
			default:
				return 0
			}
		})

		fmt.Fprintln(w, "\nStatistics per query")
		for _, q := range keys {
			var lost int
			var sumRTT int64

			for _, s := range spr[q] {
				if s.err != nil || s.m == nil {
					lost++
				}
				sumRTT += s.rtt.Milliseconds()
			}

			l := len(spr[q])
			r := 100 * float64(l-lost) / float64(l)
			fmt.Fprintf(w, "  %s\t%10d reqs\t%5.1f%% OK\tRTT: %d ms\n", q.ToString(), l, r, sumRTT/int64(l))
		}
	}

	w.Flush()
}
