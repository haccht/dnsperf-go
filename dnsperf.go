package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/miekg/dns"
)

type DNSPerf struct {
	mu   sync.Mutex
	sent int
	lost int
	stat []*DNSPerfResult

	Server string
	Client *dns.Client
}

func NewDNSPerf(server string) *DNSPerf {
	return &DNSPerf{
		Server: server,
		Client: new(dns.Client),
	}
}

type DNSPerfRequest struct {
	rName string
	rType uint16
}

type DNSPerfResult struct {
	q   *DNSPerfRequest
	r   *dns.Msg
	t   time.Time
	ok  bool
	rtt time.Duration
}

func (p *DNSPerf) Query(ctx context.Context, q *DNSPerfRequest) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(q.rName), q.rType)

	start := time.Now()
	r, _, err := p.Client.Exchange(m, p.Server)

	ok := false
	if r != nil && r.Rcode == dns.RcodeSuccess && err == nil {
		ok = true
	}

	s := &DNSPerfResult{q: q, r: r, t: start, ok: ok, rtt: time.Since(start)}

	p.mu.Lock()
	p.sent++
	if !s.ok {
		p.lost++
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
	sent := p.sent
	lost := p.lost
	stat := p.stat
	p.mu.Unlock()

	success := sent - lost
	successRate := (float64(success) / float64(sent)) * 100

	var sumRtt, minRtt, maxRtt int64
	rcodeCount := make(map[string]int)
	for _, s := range stat {
		rtt := s.rtt.Milliseconds()
		sumRtt += rtt

		if minRtt > rtt || minRtt == 0 {
			minRtt = rtt
		}

		if maxRtt < rtt || maxRtt == 0 {
			maxRtt = rtt
		}

		if s.r != nil {
			rcode := dns.RcodeToString[s.r.Rcode]
			rcodeCount[rcode]++
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent: \t%10d reqs\n", sent)
	fmt.Fprintf(w, "  Queries completed: \t%10d reqs\t%5.1f%%\n", sent-lost, successRate)
	fmt.Fprintf(w, "  Queries lost: \t%10d reqs\t%5.1f%%\n", lost, 100.0-successRate)
	fmt.Fprintf(w, "  Queries per seconds: \t%10.1f q/s\n", float64(sent)/cfg.Duration.Seconds())
	if success > 0 {
		avgRtt := sumRtt / int64(success)
		fmt.Fprintf(w, "  Latency min: \t%10d ms\n", minRtt)
		fmt.Fprintf(w, "  Latency avg: \t%10d ms\n", avgRtt)
		fmt.Fprintf(w, "  Latency max: \t%10d ms\n", maxRtt)
	}

	if cfg.ShowDetail {
		fmt.Fprintln(w, "\nStatistics per Rcode")
		for rcode, count := range rcodeCount {
			fmt.Fprintf(w, "  %8s count: \t%10d reqs\n", rcode, count)
		}

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
			kx := fmt.Sprintf("%s %s", x.rName, dns.TypeToString[x.rType])
			ky := fmt.Sprintf("%s %s", y.rName, dns.TypeToString[y.rType])
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
			var sumRTT int64
			var success float64

			for _, s := range spr[q] {
				if s.ok {
					success++
				}
				sumRTT += s.rtt.Milliseconds()
			}

			l := len(spr[q])
			r := 100 * success / float64(l)
			k := fmt.Sprintf("%s %s", q.rName, dns.TypeToString[q.rType])
			fmt.Fprintf(w, "  %s\t%10d reqs\t%5.1f%% OK\tRTT: %d ms\n", k, l, r, sumRTT/int64(l))
		}
	}

	w.Flush()
}
