package main

import (
	"io"
	"log"
	"net"
	"time"
)

// Implement the net.Conn interface.
type RetryableConn struct {
	l net.Listener
	c net.Conn
}

func NewRetryableConn(listener net.Listener) (conn *RetryableConn, err error) {
	newConn, err := listener.Accept()
	conn = &RetryableConn{
		l: listener,
		c: newConn,
	}
	return conn, err
}

func (conn *RetryableConn) Read(b []byte) (n int, err error) {
	n, err = conn.c.Read(b)
	if err == io.EOF {
		log.Println("Reconnecting")
		conn.c.Close()
		conn.c, err = conn.l.Accept()
		if err != nil {
			return 0, err
		}
		return conn.c.Read(b)
	}
	return n, err
}

func (conn *RetryableConn) Write(b []byte) (n int, err error) {
	n, err = conn.c.Write(b)
	return n, err
}

func (conn *RetryableConn) Close() error {
	return conn.c.Close()
}

func (conn *RetryableConn) LocalAddr() net.Addr {
	return conn.c.LocalAddr()
}

func (conn *RetryableConn) RemoteAddr() net.Addr {
	return conn.c.RemoteAddr()
}

func (conn *RetryableConn) SetDeadline(t time.Time) (err error) {
	return conn.c.SetDeadline(t)
}

func (conn *RetryableConn) SetReadDeadline(t time.Time) error {
	return conn.c.SetReadDeadline(t)
}

func (conn *RetryableConn) SetWriteDeadline(t time.Time) error {
	return conn.c.SetWriteDeadline(t)
}
