package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/miekg/dns"
)

type Options struct {
	Input         string        `short:"d" long:"input" description:"Path to query list file (required)" required:"true"`
	Target        string        `short:"s" long:"server" description:"DNS server address" default:"127.0.0.1:53"`
	Transport     string        `short:"m" long:"transport" description:"Network transport mode" choice:"udp" choice:"tcp" default:"udp"`
	Timeout       time.Duration `short:"t" long:"timeout" description:"Timeout for query completion" default:"1s"`
	Duration      time.Duration `short:"l" long:"duration" description:"Total benchmark duration" default:"10s"`
	Loops         int           `short:"n" long:"loops" description:"Maximum passes over the input list (0 = unlimited)" default:"0"`
	Concurrency   int           `short:"c" long:"workers" description:"Number of concurrent workers" default:"1"`
	Rate          int           `short:"Q" long:"qps" description:"Global query-per-seconds limit" default:"1"`
	StatsInterval time.Duration `short:"S" long:"realtime-stats" description:"Print stats every N seconds (0s = disable)" default:"0s"`
	StatsPerQuery bool          `short:"p" long:"per-query-stats" description:"Print stats per queries (default: false)"`
	Shuffle       bool          `short:"r" long:"shuffle" description:"Shuffle input (default: false)"`

	Queries []*query `no-flag:"true"`
}

func loadOptions() (*Options, error) {
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			os.Exit(0)
		}
		return nil, err
	}

	file, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid line: Expected 'qname qtype', but was '%s'", line)
		}

		qnameStr := parts[0]
		qtypeStr := strings.ToUpper(parts[1])

		qtype, ok := dns.StringToType[qtypeStr]
		if !ok {
			return nil, fmt.Errorf("Invalid line: Unsupported qtype '%s'", parts[1])
		}

		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(qnameStr), qtype)

		query := &query{
			msg: m,
			key: fmt.Sprintf("%s %s", qnameStr, qtypeStr),
		}
		opts.Queries = append(opts.Queries, query)
	}

	return &opts, scanner.Err()
}
