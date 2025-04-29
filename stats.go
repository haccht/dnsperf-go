package main

import (
	"bytes"
	"fmt"
	"maps"
	"math"
	"slices"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/miekg/dns"
)

type counters struct {
	sent   uint64
	lost   uint64
	rcodes map[int]uint64
}

func newCounters() counters {
	return counters{rcodes: make(map[int]uint64)}
}

type stats struct {
	c  counters
	p  counters
	mc map[string]counters
	pq bool
	mu sync.Mutex

	rttSum     uint64
	rttSqSum   uint64
	rttMin     uint64
	rttMax     uint64
	reqSizeSum uint64
	resSizeSum uint64
}

func newStats(statsPQ bool) *stats {
	return &stats{
		c:  newCounters(),
		p:  newCounters(),
		mc: make(map[string]counters),
		pq: statsPQ,
	}
}

func (s *stats) Record(q *query, r *response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.c.sent++
	s.reqSizeSum += uint64(q.msg.Len())

	if r.msg == nil {
		s.c.lost++
	} else {
		s.c.rcodes[r.rcode]++
		s.resSizeSum += uint64(r.msg.Len())
	}

	rtt := uint64(r.rtt.Microseconds())
	s.rttSum += rtt
	s.rttSqSum += rtt * rtt
	if prev := s.rttMin; prev == 0 || rtt < prev {
		s.rttMin = rtt
	}
	if prev := s.rttMax; prev == 0 || rtt > prev {
		s.rttMax = rtt
	}

	if s.pq {
		c, ok := s.mc[q.key]
		if !ok {
			c = newCounters()
		}

		c.sent++
		if r.msg == nil {
			c.lost++
		} else {
			c.rcodes[r.rcode]++
		}
		s.mc[q.key] = c
	}
}

func (s *stats) Realtime(interval time.Duration) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var buf bytes.Buffer
	qps := float64(s.c.sent-s.p.sent) / interval.Seconds()

	fmt.Fprintf(&buf, "\tQPS=%.1fq/s", qps)
	fmt.Fprintf(&buf, "\tSent=%-5d", s.c.sent-s.p.sent)
	fmt.Fprintf(&buf, "\tLoss=%-5d", s.c.lost-s.p.lost)
	for rcode := range len(dns.RcodeToString) {
		if cnt := s.c.rcodes[rcode] - s.p.rcodes[rcode]; cnt > 0 {
			fmt.Fprintf(&buf, "\t%s=%-5d", dns.RcodeToString[rcode], cnt)
			s.p.rcodes[rcode] = s.c.rcodes[rcode]
		}
	}
	s.p.sent = s.c.sent
	s.p.lost = s.c.lost

	return buf.String()
}

func (s *stats) Overall(elapsed time.Duration) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 8, 0, '\t', 0)

	received := s.c.sent - s.c.lost
	lossRate := 100 * float64(s.c.lost) / float64(s.c.sent)

	rttAvg := float64(s.rttSum) / float64(received)
	rttStd := math.Sqrt(float64(s.rttSqSum)/float64(received) - rttAvg*rttAvg)

	fmt.Fprintln(w, "\nStatistics")
	fmt.Fprintf(w, "  Queries sent:\t%10d reqs\n", s.c.sent)
	fmt.Fprintf(w, "  Queries completed:\t%10d reqs\t%5.1f%%\n", received, 100-lossRate)
	fmt.Fprintf(w, "  Queries lost:\t%10d reqs\t%5.1f%%\n", s.c.lost, lossRate)
	fmt.Fprintf(w, "  Queries per seconds:\t%10.1f q/s\n", float64(s.c.sent)/elapsed.Seconds())
	fmt.Fprintf(w, "  Run time:\t%10.1f sec\n", elapsed.Seconds())
	fmt.Fprintf(w, "  Latency(min):\t%10.2f msec\n", float64(s.rttMin)/1000)
	fmt.Fprintf(w, "  Latency(avg):\t%10.2f msec\n", rttAvg/1000)
	fmt.Fprintf(w, "  Latency(max):\t%10.2f msec\n", float64(s.rttMax)/1000)
	fmt.Fprintf(w, "  Latency(stddev):\t%10.2f msec\n", rttStd/1000)

	if received > 0 {
		fmt.Fprintf(w, "  Request size(avg):\t%10.1f bytes\n", float64(s.reqSizeSum)/float64(s.c.sent))
		fmt.Fprintf(w, "  Response size(avg):\t%10.1f bytes\n", float64(s.resSizeSum)/float64(received))

		fmt.Fprintln(w, "\nStatistics per Rcode")
		for rcode := range len(dns.RcodeToString) {
			if cnt := s.c.rcodes[rcode]; cnt > 0 {
				fmt.Fprintf(w, "  %s count:\t%10d reqs\n", dns.RcodeToString[rcode], cnt)
			}
		}
	}

	if received > 0 && s.pq {
		fmt.Fprintln(w, "\nStatistics per query")
		keys := slices.Sorted(maps.Keys(s.mc))
		for _, key := range keys {
			c := s.mc[key]
			fmt.Fprintf(w, "  [%s] \tSent=%d\tLoss=%d", key, c.sent, c.lost)
			for rcode := range len(dns.RcodeToString) {
				if cnt := c.rcodes[rcode]; cnt > 0 {
					fmt.Fprintf(w, "\t%s=%d", dns.RcodeToString[rcode], cnt)
				}
			}
			fmt.Fprintln(w)
		}
	}

	w.Flush()
	return buf.String()
}
