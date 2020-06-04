package main

import (
	"golang.org/x/net/dns/dnsmessage"
	"sync"
)

type Store struct {
	sync.RWMutex
	data map[string]dnsmessage.Message
}

func NewStore() *Store {
	return &Store{data: make(map[string]dnsmessage.Message)}
}

func (s *Store) Get(domain string) (dnsmessage.Message, bool) {
	s.RLock()
	message, ok := s.data[domain]
	s.RUnlock()

	return message, ok
}

func (s *Store) Set(domain string, message dnsmessage.Message) {
	s.Lock()
	s.data[domain] = message
	s.Unlock()
}
