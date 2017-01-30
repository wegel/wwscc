package main

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// Implement the net.Conn interface.
// All data are transfered in binary stream.
type Conn struct {
	client *Client
	r      io.Reader
}

func NewConn(client *Client) (conn *Conn, err error) {
	conn = &Conn{
		client: client,
	}
	return
}

// Read reads data from the connection.
// Read can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (conn *Conn) Read(b []byte) (n int, err error) {
	var messageType int
	if conn.r == nil {
		// New message
		var r io.Reader
		for {
			messageType, r, err = conn.client.NextReader()
			if err != nil {
				return
			}
			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}

			conn.r = r
			break
		}
	}

	//TODO: need lock?
	n, err = conn.r.Read(b)
	if err != nil {
		if err == io.EOF {
			// Message finished
			conn.r = nil
			err = nil
		}
	}
	return
}

// Write writes data to the connection.
// Write can be made to time out and return a Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetWriteDeadline.
func (conn *Conn) Write(b []byte) (n int, err error) {
	var w io.WriteCloser

	w, err = conn.client.NextWriter(websocket.BinaryMessage)

	if err != nil {
		return
	}

	n, err = w.Write(b)

	if err != nil {
		return
	}

	err = w.Close()
	return
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (conn *Conn) Close() error {
	return conn.client.ws.Close()
}

// LocalAddr returns the local network address.
func (conn *Conn) LocalAddr() net.Addr {
	return conn.client.ws.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (conn *Conn) RemoteAddr() net.Addr {
	return conn.client.ws.RemoteAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail with a timeout (see type Error) instead of
// blocking. The deadline applies to all future I/O, not just
// the immediately following call to Read or Write.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (conn *Conn) SetDeadline(t time.Time) (err error) {
	err = conn.client.SetReadDeadline(t)
	if err != nil {
		return
	}

	err = conn.client.SetWriteDeadline(t)
	return err
}

// SetReadDeadline sets the deadline for future Read calls.
// A zero value for t means Read will not time out.
func (conn *Conn) SetReadDeadline(t time.Time) (err error) {
	err = conn.client.SetReadDeadline(t)
	return err
}

// SetWriteDeadline sets the deadline for future Write calls.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (conn *Conn) SetWriteDeadline(t time.Time) (err error) {
	err = conn.client.SetWriteDeadline(t)
	return err
}
