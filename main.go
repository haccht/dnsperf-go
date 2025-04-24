package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
    "os/signal"
	"syscall"
	"sync"
	"time"
)

func run(cfg *Config) {
	log.Printf("Sending queries to %s", cfg.Server)
	log.Printf("Duration=%s  RateLimit=%dq/s  Workers=%d", cfg.Duration.String(), cfg.QPS, cfg.Workers)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	perf := NewDNSPerf(cfg.Server, cfg.Protocol, cfg.Timeout)
	reqCh := make(chan int, cfg.QPS)

	// start workers
	var wg sync.WaitGroup
	for range cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case reqIndex := <-reqCh:
					req := cfg.Requests[reqIndex]
					perf.Perform(req)
				}
			}
		}()
	}

	// qps counter
	if cfg.QPSInterval.Seconds() > 0 {
		wg.Add(1)
		go func() {
			ticker := time.NewTicker(cfg.QPSInterval)
			defer ticker.Stop()
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					log.Println(perf.Tick(cfg))
				}
			}
		}()
	}

	// rate limit
	go func() {
		policer := time.NewTicker(time.Second / time.Duration(cfg.QPS))
		defer policer.Stop()

		maxRequests := cfg.MaxSweep * len(cfg.Requests)
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				close(reqCh)
				return
			case <-policer.C:
				if maxRequests > 0 && i >= maxRequests {
					cancel()
					continue
				}

				if cfg.Shuffle {
					reqCh <- rand.Intn(len(cfg.Requests))
				} else {
					reqCh <- i % len(cfg.Requests)
				}
			}
		}
	}()

	go func() {
        sigs := make(chan os.Signal, 1)
        signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs
		cancel()
	}()

	<-ctx.Done()

	wg.Wait()
	log.Println("Performance test completed")

	fmt.Print(perf.Statistics(cfg))
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	run(cfg)
}
