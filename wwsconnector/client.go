package main

import (
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	hub        *Hub
	ws         *websocket.Conn
	otherSide  *Client
	channelID  uuid.UUID
	remoteType string
	params     map[string][]string
	wmu        sync.Mutex
	rmu        sync.Mutex
}

func (c *Client) WriteMessage(msgType int, message []byte) (err error) {
	c.wmu.Lock()
	err = c.ws.WriteMessage(msgType, message)
	c.wmu.Unlock()
	return
}

func (c *Client) NextWriter(msgType int) (w io.WriteCloser, err error) {
	c.wmu.Lock()
	w, err = c.ws.NextWriter(msgType)
	c.wmu.Unlock()
	return
}

func (c *Client) ReadMessage() (msgType int, message []byte, err error) {
	c.rmu.Lock()
	msgType, message, err = c.ws.ReadMessage()
	c.rmu.Unlock()
	return
}

func (c *Client) NextReader() (msgType int, r io.Reader, err error) {
	c.rmu.Lock()
	msgType, r, err = c.ws.NextReader()
	c.rmu.Unlock()
	return
}

func (c *Client) SetWriteDeadline(t time.Time) (err error) {
	c.wmu.Lock()
	err = c.ws.SetWriteDeadline(t)
	c.wmu.Unlock()
	return
}

func (c *Client) SetReadDeadline(t time.Time) (err error) {
	c.rmu.Lock()
	err = c.ws.SetReadDeadline(t)
	c.rmu.Unlock()
	return
}
