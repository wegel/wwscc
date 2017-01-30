// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// 'Connect on Write' net.Conn wrapper
type COWConn struct {
	ready     chan struct{}
	tcp       net.Conn
	addr      *net.TCPAddr
	connected bool
}

func NewCOWConn(remote string, ready chan struct{}) (conn *COWConn, err error) {
	addr, err := net.ResolveTCPAddr("tcp", remote)
	if err != nil {
		println("ResolveTCPAddr failed:", err.Error())
		return nil, err
	}

	return &COWConn{ready: ready, addr: addr, connected: false}, nil
}

func (conn *COWConn) Read(b []byte) (n int, err error) {
	if conn.tcp == nil || conn.connected == false {
		return 0, fmt.Errorf("COWConn: Read: tcp not connected yet")
	}
	return conn.tcp.Read(b)
}

func (conn *COWConn) Write(b []byte) (n int, err error) {
	if !conn.connected {
		log.Println("Connecting to", conn.addr.String())
		conn.tcp, err = net.DialTCP("tcp", nil, conn.addr)
		if err != nil {
			println("Dial failed:", err.Error())
			os.Exit(1)
		}
		conn.connected = true
		conn.ready <- struct{}{}
	}

	return conn.tcp.Write(b)
}

func (conn *COWConn) Close() error {
	if conn.tcp == nil || conn.connected == false {
		return nil
	}
	return conn.tcp.Close()
}

func (conn *COWConn) LocalAddr() net.Addr {
	if conn.tcp == nil {
		return nil
	}
	return conn.tcp.LocalAddr()
}

func (conn *COWConn) RemoteAddr() net.Addr {
	if conn.tcp == nil {
		return nil
	}
	return conn.tcp.RemoteAddr()
}

func (conn *COWConn) SetDeadline(t time.Time) (err error) {
	if conn.tcp == nil || conn.connected == false {
		return fmt.Errorf("COWConn: SetDeadline: tcp not connected yet")
	}
	return conn.tcp.SetDeadline(t)
}

func (conn *COWConn) SetReadDeadline(t time.Time) error {
	if conn.tcp == nil || conn.connected == false {
		return fmt.Errorf("COWConn: SetReadDeadline: tcp not connected yet")
	}
	return conn.tcp.SetReadDeadline(t)
}

func (conn *COWConn) SetWriteDeadline(t time.Time) error {
	if conn.tcp == nil || conn.connected == false {
		return fmt.Errorf("COWConn: SetWriteDeadline: tcp not connected yet")
	}
	return conn.tcp.SetWriteDeadline(t)
}
