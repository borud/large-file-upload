package main

import (
	"log/slog"
	"net"
	"time"
)

type timeoutListener struct {
	net.Listener
	Timeout time.Duration
}

type timeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (l *timeoutListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &timeoutConn{Conn: conn, timeout: l.Timeout}, nil
}

func (c *timeoutConn) Read(b []byte) (int, error) {
	slog.Info("reading with timeout", "timeout", c.timeout)
	err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
	if err != nil {
		slog.Error("error setting deadline", "err", err)
	}
	return c.Conn.Read(b)
}
