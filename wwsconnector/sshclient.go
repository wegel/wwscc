// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func getSupportedCiphers() []string {
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

func sshShell(channel *Channel) {
	defer func(id uuid.UUID) {
		channel.proxy.hub.disconnected <- channel.proxy
		if r := recover(); r != nil {
			fmt.Printf("Exception handled in sshShell for channel %v: %v\n", id, r)
		}
	}(channel.id)

	username := channel.tunnel.params["username"][0]
	cols, _ := strconv.Atoi(channel.tunnel.params["cols"][0])
	rows, _ := strconv.Atoi(channel.tunnel.params["rows"][0])
	wsWrapper, err := NewConn(channel.tunnel)

	var password string
	if channel.tunnel.params["password"] != nil {
		password = channel.tunnel.params["password"][0]
	}

	authMethod := ssh.PasswordCallback(func() (string, error) {
		wsWrapper.Write([]byte(fmt.Sprintf("%s password: ", username)))

		scanner := bufio.NewScanner(wsWrapper)
		scanner.Scan()
		pwd := strings.TrimSpace(scanner.Text())

		wsWrapper.Write([]byte("\r\n"))
		return pwd, nil
	})

	if len(password) > 0 {
		authMethod = ssh.Password(password)
	}

	config := &ssh.ClientConfig{
		Config: ssh.Config{Ciphers: getSupportedCiphers()},
		User:   username,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
	}

	proxyConn, _ := NewConn(channel.proxy)
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

	channel.tunnel.ws.Close()
	channel.proxy.ws.Close()
}
