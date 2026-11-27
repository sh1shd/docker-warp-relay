package main

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type relay struct {
	listener *net.UDPConn
	upstream *net.UDPAddr
	mu       sync.Mutex
	sessions map[string]*clientSession
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
