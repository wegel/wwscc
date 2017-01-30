package main

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

func Passthrough(channel *Channel) {
	passthrough_func :=
		func(c *Client) {
			defer func(remoteType string) {
				if r := recover(); r != nil {
					fmt.Printf("passthrough handled exception for %s: %v\n", remoteType, r)
				}
			}(c.remoteType)
			//ws.SetReadDeadline(time.Now().Add(pongWait))
			//c.conn.SetPongHandler(func(string) error { /*ws.SetReadDeadline(time.Now().Add(pongWait));*/ return nil })
			for {
				msgType, message, err := c.conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						log.Printf("%s read error, msg type %v, on channel %v: %v\n", c.remoteType, msgType, c.channelId, err)
					}
					if c.otherSide != nil && c.otherSide.conn != nil {
						c.otherSide.conn.Close()
					}
					if c.conn != nil {
						c.conn.Close()
					}

					c.hub.disconnected <- c
					break
				} else {
					if c.otherSide != nil && c.otherSide.conn != nil {
						c.otherSide.conn.WriteMessage(msgType, message)
					}
				}
			}
		}

	go passthrough_func(channel.proxy)
	passthrough_func(channel.tunnel)
}
