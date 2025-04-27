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
	Filepath     string        `short:"d" long:"filepth" description:"The input data file (required)" required:"true"`
	Server       string        `short:"s" long:"server" description:"DNS server address to query" default:"127.0.0.1:53"`
	Protocol     string        `short:"m" long:"protocol" description:"Set transport mode" choice:"udp" choice:"tcp" default:"udp"`
	Timeout      time.Duration `short:"t" long:"timeout" description:"The timeout for query completion" default:"1s"`
	Duration     time.Duration `short:"l" long:"duration" description:"Run for at most this duration" default:"10s"`
	MaxSweep     int           `short:"n" long:"max-sweep" description:"Run through input at most N times"`
	Workers      int           `short:"c" long:"clients" description:"The number of concurrent clients" default:"1"`
	Shuffle      bool          `short:"r" long:"shuffle" description:"Shuffle input"`
	QPS          int           `short:"Q" long:"rate-limit" description:"Limit the number of QPS" default:"1"`
	TickInterval time.Duration `short:"S" long:"show-ticker" description:"The interval to show realtime QPS" default:"0s"`
	Verbose      bool          `short:"v" long:"verbose" description:"Print detail stats"`

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

		qnameStr := parts[0]
		qtypeStr := strings.ToUpper(parts[1])
		qtype, ok := dns.StringToType[qtypeStr]
		if !ok {
			return nil, fmt.Errorf("Invalid line: Unsupported type '%s'", parts[1])
		}

		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(qnameStr), qtype)

		key := fmt.Sprintf("%s %s", qnameStr, qtypeStr)
		request := &DNSPerfRequest{m: m, key: key}

		config.Requests = append(config.Requests, request)
	}

	return &config, scanner.Err()
}
