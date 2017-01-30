package main

import (
	"log"
	"strings"

	"./wsconn"

	"golang.org/x/crypto/ssh"

	"bufio"

	"github.com/gorilla/websocket"
)

func GetSupportedCiphers() []string {
	config := &ssh.ClientConfig{}
	config.SetDefaults()
	for _, cipher := range []string{"aes128-cbc"} {
		found := false
		for _, defaultCipher := range config.Ciphers {
			if cipher == defaultCipher {
				found = true
				break
			}
		}

		if !found {
			config.Ciphers = append(config.Ciphers, cipher)
		}
	}
	return config.Ciphers
}

func sshShell(ws *websocket.Conn, proxy *Client, user string, cols int, rows int) {

	defer func() {
		ws.Close()
	}()

	wsWrapper, err := wsconn.NewConn(ws)

	config := &ssh.ClientConfig{
		Config: ssh.Config{Ciphers: GetSupportedCiphers()},
		User:   user,
		Auth: []ssh.AuthMethod{
			ssh.PasswordCallback(func() (string, error) {
				wsWrapper.Write([]byte("Give Wégel the password: "))

				scanner := bufio.NewScanner(wsWrapper)
				scanner.Scan()

				pwd := strings.TrimSpace(scanner.Text())

				wsWrapper.Write([]byte("\r\n"))
				return pwd, nil

			}),
		},
	}

	proxyConn, _ := wsconn.NewConn(proxy.conn)
	c, chans, reqs, err := ssh.NewClientConn(proxyConn, "localhost", config)
	if err != nil {
		log.Println("Error NewClientConn:", err)
		return
	}

	client := ssh.NewClient(c, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		log.Println("Failed to create session: ", err)
		return
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	log.Println("Requesting pseudo-terminal")
	if err = session.RequestPty("xterm", rows, cols, modes); err != nil {
		log.Println("request for pseudo terminal failed: ", err)
		return
	}

	session.Stdout = wsWrapper
	session.Stderr = wsWrapper
	session.Stdin = wsWrapper

	if err := session.Shell(); nil != err {
		log.Println("Unable to execute command:", err)
		return
	}

	log.Println("Waiting")
	if err := session.Wait(); nil != err {
		log.Println("Unable to execute command:", err)
	}
}
