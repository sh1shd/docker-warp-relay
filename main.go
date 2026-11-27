package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
