package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

func runBenchmark(opts *Options) {
	log.Printf("Sending queries to %s/%s", opts.Target, opts.Transport)
	log.Printf("Duration=%s  RateLimit=%dq/s  Workers=%d", opts.Duration.String(), opts.Rate, opts.Concurrency)

	ctx, cancel := context.WithTimeout(context.Background(), opts.Duration)
	defer cancel()

	rs := newResolver(opts.Target, opts.Transport, opts.Timeout)
	start := time.Now()
	stats := newStats(opts.StatsPerQuery)
	jobCh := make(chan *query)

	// worker
	var wg sync.WaitGroup
	for range opts.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for q := range jobCh {
				r, _ := rs.Resolve(q)
				stats.Record(q, r)
			}
		}()
	}

	// feeder
	limiter := rate.NewLimiter(rate.Limit(opts.Rate), 1)
	go func() {
		defer close(jobCh)

		rand.Seed(time.Now().UnixNano())
		idxFn := func(idx int) int { return idx % len(opts.Queries) }
		if opts.Shuffle {
			idxFn = func(idx int) int { return rand.Intn(len(opts.Queries)) }
		}

		maxTotal := opts.Loops * len(opts.Queries)
		for i := 0; ; i++ {
			if opts.Loops > 0 && i >= maxTotal {
				return
			}
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			jobCh <- opts.Queries[idxFn(i)]
		}
	}()

	// ticker
	if opts.StatsInterval > 0 {
		ticker := time.NewTicker(opts.StatsInterval)

		go func() {
			for {
				select {
				case <-ctx.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					log.Println(stats.Realtime(opts.StatsInterval))
				}
			}
		}()
	}

	// signal
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		cancel()
	}()

	<-ctx.Done()
	wg.Wait()
	log.Println("Performance test completed")
	fmt.Println(stats.Overall(time.Since(start)))
}
