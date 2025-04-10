package main

import (
	"fmt"
	"sync"
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
	name string
	Type uint16
}

type DNSPerfResult struct {
	q   *DNSPerfRequest
	r   *dns.Msg
	t   time.Time
	ok  bool
	rtt time.Duration
}

func (p *DNSPerf) Query(q *DNSPerfRequest) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(q.name), q.Type)

	start := time.Now()
	r, _, err := p.Client.Exchange(m, p.Server)

	ok := false
	if r != nil && r.Rcode == dns.RcodeSuccess && err == nil {
		ok = true
	}

	s := &DNSPerfResult{q: q, r: r, t: start, ok: ok, rtt: time.Since(start)}

	p.mu.Lock()
	p.sent++
	if !ok {
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

		rcode := dns.RcodeToString[s.r.Rcode]
		rcodeCount[rcode]++
	}

	fmt.Println("\nStatistics:")
	fmt.Printf("  Queries sent:         %d\n", sent)
	fmt.Printf("  Queries completed:    %d  %5.1f%%\n", sent-lost, successRate)
	fmt.Printf("  Queries lost:         %d  %5.1f%%\n", lost, 100.0-successRate)
	fmt.Printf("  Queries per seconds:  %.1fq/s\n", float64(sent)/cfg.Duration.Seconds())

	if success > 0 {
		avgRtt := sumRtt/int64(success)
		fmt.Printf("  Latency(min/avg/max): %dms / %dms / %dms\n", minRtt, avgRtt, maxRtt)
	}

	fmt.Println()
	for rcode, count := range rcodeCount {
		fmt.Printf("  %-8s count:       %d\n", rcode, count)
	}
}
