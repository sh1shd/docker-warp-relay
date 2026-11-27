package relay

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sh1shd/docker-warp-relay/session"
	"github.com/sh1shd/docker-warp-relay/utils"
)

type Relay struct {
	listener *net.UDPConn
	upstream *net.UDPAddr
	mu       sync.Mutex
	sessions map[string]*session.Session
}

func New(listener *net.UDPConn, upstream *net.UDPAddr) *Relay {
	return &Relay{
		listener: listener,
		upstream: upstream,
		sessions: make(map[string]*session.Session),
	}
}

func (r *Relay) GetSession(client *net.UDPAddr) (*session.Session, error) {
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
	session := session.New(conn, client)
	r.sessions[key] = session
	go r.readUpstream(key, session)
	return session, nil
}

func (r *Relay) readUpstream(key string, session *session.Session) {
	defer session.Close()
	buf := make([]byte, 65535)
	for {
		n, err := session.Conn().Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				r.RemoveSession(key, session)
				return
			}
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("failed read upstream response for %s (%v)", session.ClientAddr(), err)
			}
			return
		}
		_ = session.Conn().SetReadDeadline(time.Now().Add(2 * time.Minute))
		if _, err := r.listener.WriteToUDP(buf[:n], session.ClientAddr()); err != nil {
			log.Printf("failed send response to %s (%v)", session.ClientAddr(), err)
		}
	}
}

func (r *Relay) RemoveSession(key string, session *session.Session) {
	r.mu.Lock()
	if r.sessions[key] == session {
		delete(r.sessions, key)
	}
	r.mu.Unlock()
	session.Close()
}

func (r *Relay) CloseAll() {
	_ = r.listener.Close()
	r.mu.Lock()
	sessions := make([]*session.Session, 0, len(r.sessions))
	for key, session := range r.sessions {
		delete(r.sessions, key)
		sessions = append(sessions, session)
	}
	r.mu.Unlock()
	for _, session := range sessions {
		session.Close()
	}
	_ = utils.CloneUDPAddr
}
