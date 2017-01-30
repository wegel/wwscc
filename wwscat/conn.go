package main

import (
	"net"
	"os"
	"time"
)

// Implement the net.Conn interface.
type Conn struct {
	addr net.Addr
}

func NewConn() (conn *Conn, err error) {
	addr, _ := net.ResolveTCPAddr("tcp", "localhost")
	conn = &Conn{
		addr: addr,
	}
	return
}

func (conn *Conn) Read(b []byte) (n int, err error) {
	return os.Stdin.Read(b)
}

func (conn *Conn) Write(b []byte) (n int, err error) {
	return os.Stdout.Write(b)
}

func (conn *Conn) Close() error {
	return nil
}

func (conn *Conn) LocalAddr() net.Addr {
	return conn.addr
}

func (conn *Conn) RemoteAddr() net.Addr {
	return conn.addr
}

func (conn *Conn) SetDeadline(t time.Time) (err error) {
	return nil
}

func (conn *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
