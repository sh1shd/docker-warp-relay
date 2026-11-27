package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const idleTimeout = 2 * time.Minute

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

type relay struct {
	listener *net.UDPConn
	upstream *net.UDPAddr
	mu       sync.Mutex
	sessions map[string]*clientSession
}

func main() {
	log.SetPrefix("[docker-warp-relay] ")

	srcPort, err := envPort("SRC_PORT", "4500")
	if err != nil {
		log.Fatal(err)
	}
	dstPort, err := envPort("DST_PORT", "4500")
	if err != nil {
		log.Fatal(err)
	}
	dstHost := strings.TrimSpace(env("DST_HOST", "engage.cloudflareclient.com"))
	if dstHost == "" {
		log.Fatal("DST_HOST must not be empty")
	}

	upstream, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(dstHost, strconv.Itoa(dstPort)))
	if err != nil {
		log.Fatalf("resolve upstream %q: %v", dstHost, err)
	}
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: srcPort})
	if err != nil {
		log.Fatalf("listen on UDP port %d: %v", srcPort, err)
	}

	r := &relay{
		listener: listener,
		upstream: upstream,
		sessions: make(map[string]*clientSession),
	}
	defer r.closeAll()

	ctx, stop := signal.NotifyContext(nil, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("listening on udp/%d and relaying to %s", srcPort, upstream)
	buf := make([]byte, 65535)
	for {
		_ = listener.SetReadDeadline(time.Now().Add(time.Second))
		n, client, err := listener.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					log.Print("stopping")
					return
				default:
					continue
				}
			}
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("receive client packet: %v", err)
			continue
		}

		session, err := r.getSession(client)
		if err != nil {
			log.Printf("open upstream session for %s: %v", client, err)
			continue
		}
		if err := session.send(buf[:n]); err != nil {
			log.Printf("send packet for %s: %v", client, err)
			r.removeSession(client.String(), session)
		}
	}
}

func (r *relay) getSession(client *net.UDPAddr) (*clientSession, error) {
	key := client.String()
	r.mu.Lock()
	defer r.mu.Unlock()
	if session := r.sessions[key]; session != nil {
		return session, nil
	}

	conn, err := net.DialUDP("udp4", nil, r.upstream)
	if err != nil {
		return nil, err
	}
	session := &clientSession{conn: conn, client: cloneUDPAddr(client)}
	r.sessions[key] = session
	go r.readUpstream(key, session)
	return session, nil
}

func (r *relay) readUpstream(key string, session *clientSession) {
	defer session.close()
	buf := make([]byte, 65535)
	for {
		n, err := session.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				r.removeSession(key, session)
				return
			}
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("read upstream response for %s: %v", session.client, err)
			}
			return
		}
		_ = session.conn.SetReadDeadline(time.Now().Add(idleTimeout))
		if _, err := r.listener.WriteToUDP(buf[:n], session.client); err != nil {
			log.Printf("send response to %s: %v", session.client, err)
		}
	}
}

func (r *relay) removeSession(key string, session *clientSession) {
	r.mu.Lock()
	if r.sessions[key] == session {
		delete(r.sessions, key)
	}
	r.mu.Unlock()
	session.close()
}

func (r *relay) closeAll() {
	_ = r.listener.Close()
	r.mu.Lock()
	sessions := make([]*clientSession, 0, len(r.sessions))
	for key, session := range r.sessions {
		delete(r.sessions, key)
		sessions = append(sessions, session)
	}
	r.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	return &net.UDPAddr{IP: append(net.IP(nil), addr.IP...), Port: addr.Port, Zone: addr.Zone}
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envPort(key, fallback string) (int, error) {
	value := env(key, fallback)
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be an integer from 1 to 65535", key)
	}
	return port, nil
}
