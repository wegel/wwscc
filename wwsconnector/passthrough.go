package main

import (
	"fmt"

	"github.com/gorilla/websocket"
)

func Passthrough(channel *Channel) {
	passthroughFunc :=
		func(c *Client) {
			defer func(remoteType string) {
				c.hub.disconnected <- c
				if r := recover(); r != nil {
					fmt.Printf("Passthrough handled exception for %s: %v\n", remoteType, r)
				}
			}(c.remoteType)
			for {
				msgType, message, err := c.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
						panic(fmt.Errorf("%s read error, msg type %v, on channel %v: %v\n", c.remoteType, msgType, channel.id, err))
					}
					return
				}
				if c.otherSide != nil && c.otherSide.ws != nil {
					c.otherSide.WriteMessage(msgType, message)
				}
			}
		}

	go passthroughFunc(channel.proxy)
	passthroughFunc(channel.tunnel)
}
