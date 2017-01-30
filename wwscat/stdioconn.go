// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"net"
	"os"
	"time"
)

// Implement the net.Conn interface.
type StdioConn struct {
	addr net.Addr
}

func NewStdioConn(ready chan<- struct{}) (conn *StdioConn, err error) {
	addr, _ := net.ResolveTCPAddr("tcp", "localhost")
	conn = &StdioConn{
		addr: addr,
	}
	ready <- struct{}{}
	return
}

func (conn *StdioConn) Read(b []byte) (n int, err error) {
	return os.Stdin.Read(b)
}

func (conn *StdioConn) Write(b []byte) (n int, err error) {
	return os.Stdout.Write(b)
}

func (conn *StdioConn) Close() error {
	return nil
}

func (conn *StdioConn) LocalAddr() net.Addr {
	return conn.addr
}

func (conn *StdioConn) RemoteAddr() net.Addr {
	return conn.addr
}

func (conn *StdioConn) SetDeadline(t time.Time) (err error) {
	return nil
}

func (conn *StdioConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *StdioConn) SetWriteDeadline(t time.Time) error {
	return nil
}
