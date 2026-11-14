package main

import (
	"net"
	"sync"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestStoreGetConcurrentExpiry(t *testing.T) {
	const first, second = "first.example.", "second.example."
	s := &Store{data: map[string]entry{first: {}, second: {}}}

	getExpired := func(domain string) {
		if _, ok := s.Get(domain); ok {
			t.Errorf("Get(%q) returned an expired entry", domain)
		}
	}
	runConcurrently(func() {
		getExpired(first)
	}, func() {
		getExpired(second)
	})

	if got := len(s.data); got != 0 {
		t.Errorf("cache still contains %d expired entries", got)
	}
}

func TestDNSQueryConcurrentForwarders(t *testing.T) {
	previousTTL := ttl
	cacheTTL := int64(60)
	ttl = &cacheTTL
	t.Cleanup(func() { ttl = previousTTL })

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	// Exercise Query without sending traffic to the upstream resolver.
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	client := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5300}
	const requestDomain, responseDomain = "request.example.", "response.example."
	s := &DNSService{
		conn:  conn,
		store: &Store{data: make(map[string]entry)},
		forwarders: map[string][]Packet{
			responseDomain: {
				{addr: client, message: dnsmessage.Message{Header: dnsmessage.Header{ID: 1}}},
				{addr: client, message: dnsmessage.Message{Header: dnsmessage.Header{ID: 2}}},
				{addr: client, message: dnsmessage.Message{Header: dnsmessage.Header{ID: 3}}},
			},
		},
	}

	const count = 32
	runConcurrently(func() {
		for i := uint16(0); i < count; i++ {
			s.Query(Packet{addr: client, message: dnsmessage.Message{
				Header:    dnsmessage.Header{ID: i},
				Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(requestDomain), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}},
			}})
		}
	}, func() {
		response := Packet{message: dnsmessage.Message{
			Header:    dnsmessage.Header{ID: 2, Response: true},
			Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(responseDomain), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}},
		}}
		for i := 0; i < count; i++ {
			s.Query(response)
		}
	})

	if got := len(s.forwarders[requestDomain]); got != count {
		t.Errorf("pending requests = %d, want %d", got, count)
	}
	pending := s.forwarders[responseDomain]
	if len(pending) != 2 || pending[0].message.ID != 1 || pending[1].message.ID != 3 {
		t.Errorf("pending responses = %v, want only IDs 1 and 3", pending)
	}
}

func runConcurrently(first, second func()) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, run := range []func(){first, second} {
		wg.Add(1)
		go func(run func()) {
			defer wg.Done()
			<-start
			run()
		}(run)
	}
	close(start)
	wg.Wait()
}
