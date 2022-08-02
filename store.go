package main

import (
	"golang.org/x/net/dns/dnsmessage"

	"log"
	"sync"
	"time"
)

type Store struct {
	sync.RWMutex
	data map[string]entry
}

type entry struct {
	message   dnsmessage.Message
	ttl       int64
	createdAt int64
}

var store *Store

func init() {
	store = &Store{data: make(map[string]entry)}
}

func (s *Store) Get(domain string) (dnsmessage.Message, bool) {
	s.Lock()
	m, ok := s.data[domain]
	s.Unlock()
	// check if cache expire
	if ok && time.Now().Unix() > m.createdAt + m.ttl {
		log.Printf("cache expire, domain: %s \n", domain)
		delete(s.data, domain)
		return dnsmessage.Message{}, false
	}

	return m.message, ok
}

func (s *Store) Set(domain string, message dnsmessage.Message) {
	s.Lock()
	s.data[domain] = entry{
		message:   message,
		ttl:       *ttl,
		createdAt: time.Now().Unix(),
	}
	s.Unlock()
}

func (s *Store) Delete(domain string) {
	s.Lock()
	delete(s.data, domain)
	s.Unlock()
}

func (s *Store) Flush() {
	s.Lock()
	s.data = make(map[string]entry)
	s.Unlock()
}