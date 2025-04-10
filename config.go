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

type Config struct {
	Server      string        `short:"s" long:"server" description:"DNS server address to query" default:"127.0.0.1:53"`
	Filepath    string        `short:"d" long:"filepth" description:"The input data file (required)" required:"true"`
	Duration    time.Duration `short:"l" long:"duration" description:"Run for at most this duration - e.g. 10s, 1m" default:"10s"`
	MaxSweep    int           `short:"n" long:"max-sweep" description:"Run through input at most N times - 0 for unlimited" default:"0"`
	Workers     int           `short:"w" long:"workers" description:"The number of concurrent workers" default:"1"`
	Shuffle     bool          `short:"r" long:"shuffle" description:"Shuffle input"`
	QPS         int           `short:"Q" long:"rate-limit" description:"Limit the number of queries per second" default:"10"`
	QPSInterval time.Duration `short:"S" long:"rate-interval" description:"Print qps statistics interval" default:"0s"`
	ShowDetail  bool          `short:"v" long:"detail" description:"Print detail stats"`

	Requests []*DNSPerfRequest `no-flag:"true"`
}

func LoadConfig() (*Config, error) {
	var config Config

	parser := flags.NewParser(&config, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	file, err := os.Open(config.Filepath)
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
			return nil, fmt.Errorf("Invalid line: Must be 'FQDN TYPE', but %s", line)
		}

		qname := parts[0]
		qtype, ok := dns.StringToType[strings.ToUpper(parts[1])]
		if !ok {
			return nil, fmt.Errorf("Invalid line: Unsupported type '%s'", parts[1])
		}

		request := &DNSPerfRequest{name: qname, Type: qtype}
		config.Requests = append(config.Requests, request)
	}

	return &config, scanner.Err()
}
