package main

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

func Passthrough(channel *Channel) {
	passthroughFunc :=
		func(c *Client) {
			defer func(remoteType string) {
				c.hub.disconnected <- c
				if r := recover(); r != nil {
					fmt.Printf("passthrough handled exception for %s: %v\n", remoteType, r)
				}
			}(c.remoteType)
			for {
				msgType, message, err := c.conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						log.Printf("%s read error, msg type %v, on channel %v: %v\n", c.remoteType, msgType, channel.id, err)
					}
					if c.otherSide != nil && c.otherSide.conn != nil {
						c.otherSide.conn.Close()
					}
					if c.conn != nil {
						c.conn.Close()
					}

					break
				} else {
					if c.otherSide != nil && c.otherSide.conn != nil {
						c.otherSide.conn.WriteMessage(msgType, message)
					}
				}
			}
		}

	go passthroughFunc(channel.proxy)
	passthroughFunc(channel.tunnel)
}
