package main

import (
	"net"
	"sync"
	"time"
)

type clientSession struct {
	conn   *net.UDPConn
	client *net.UDPAddr
	mu     sync.Mutex
	closed bool
}

func (s *clientSession) send(packet []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return net.ErrClosed
	}
	if err := s.conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
		return err
	}
	_, err := s.conn.Write(packet)
	return err
}

func (s *clientSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		_ = s.conn.Close()
	}
}
