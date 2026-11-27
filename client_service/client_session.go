package client_service

import (
	"net"
	"sync"
	"time"
)

const idleTimeout = 2 * time.Minute

// Session tracks a client's upstream UDP connection.
type Session struct {
	conn   *net.UDPConn
	client *net.UDPAddr
	mu     sync.Mutex
	closed bool
}

func New(conn *net.UDPConn, client *net.UDPAddr) *Session {
	return &Session{conn: conn, client: cloneUDPAddr(client)}
}

func (s *Session) Send(packet []byte) error {
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

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		_ = s.conn.Close()
	}
}

func (s *Session) Conn() *net.UDPConn { return s.conn }

func (s *Session) ClientAddr() *net.UDPAddr { return cloneUDPAddr(s.client) }

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	return &net.UDPAddr{IP: append(net.IP(nil), addr.IP...), Port: addr.Port, Zone: addr.Zone}
}
