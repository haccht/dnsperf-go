package main

import (
	"time"

	"github.com/miekg/dns"
)

type query struct {
	msg *dns.Msg
	key string
}

type response struct {
	msg   *dns.Msg
	rtt   time.Duration
	rcode int
}

type resolver struct {
	target string
	client *dns.Client
}

func newResolver(target, transport string, timeout time.Duration) *resolver {
	return &resolver{
		target: target,
		client: &dns.Client{
			Net:     transport,
			Timeout: timeout,
		},
	}
}

func (r *resolver) Resolve(q *query) (*response, error) {
	start := time.Now()
	msg, _, err := r.client.Exchange(q.msg.Copy(), r.target)

	resp := &response{msg: msg, rtt: time.Since(start)}
	if resp.msg != nil {
		resp.rcode = msg.Rcode
	}
	return resp, err
}
