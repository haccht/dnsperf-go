package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

func run(cfg *Config) {
	log.Printf("Sending queries to %s", cfg.Server)
	log.Printf("Duration=%s  RateLimit=%dq/s  Workers=%d", cfg.Duration.String(), cfg.QPS, cfg.Workers)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	client := NewDNSPerf(cfg.Server)
	reqCh := make(chan int, cfg.QPS)

	// start workers
	var wg sync.WaitGroup
	for range cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			maxRequests := cfg.MaxSweep * len(cfg.Requests)
			for {
				select {
				case <-ctx.Done():
					return
				case reqIndex := <- reqCh:
					if maxRequests > 0 && client.Sent() >= maxRequests {
						cancel()
						return
					}

					entry := cfg.Requests[reqIndex]
					client.Query(entry)
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

			var prevSent int
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					sent := client.Sent()
					lost := client.Lost()
					deltaSent := sent - prevSent
					log.Printf("  Sent: %-10d Loss: %-10d QPS: %7.1fq/s\n", sent, lost, float64(deltaSent)/cfg.QPSInterval.Seconds())

					prevSent = sent
				}
			}
		}()
	}

	// rate limit
	policer := time.NewTicker(time.Second / time.Duration(cfg.QPS))
	defer policer.Stop()
	go func() {
		reqIndex := 0
		for {
			select {
			case <-ctx.Done():
				close(reqCh)
				return
			case <-policer.C:
				if cfg.Shuffle {
					reqIndex = rand.Intn(len(cfg.Requests))
					reqCh <- reqIndex
				} else {
					reqCh <- reqIndex
					reqIndex = (reqIndex + 1) % len(cfg.Requests)
				}
			}
		}
	}()


	<-ctx.Done()
	wg.Wait()
	log.Println("Performance Test completed")

	client.PrintStats(cfg)
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	run(config)
}
