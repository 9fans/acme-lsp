package main

import (
	"log"
	"net"
	"os"
	"os/exec"
)

func runServer() net.Conn {
	p0, p1 := net.Pipe()
	cmd := exec.Command("go-langserver", "-gocodecompletion")
	cmd.Stdin = p0
	cmd.Stdout = p0
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}()
	return p1
}

func dialServer() net.Conn {
	conn, err := net.Dial("tcp", "localhost:4389")
	if err != nil {
		log.Fatal(err)
	}
	return conn
}
