package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %v <command>\n", os.Args[0])
		os.Exit(2)
	}
	conn := runServer()
	defer conn.Close()

	c, err := newLSPClient(conn)
	if err != nil {
		log.Fatalf("failed to create client: %v\n", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if os.Args[1] == "watch" {
		c.Watch()
		return
	}
	pos, _, err := getAcmePos()
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "def":
		err = c.Definition(pos)
	case "refs":
		err = c.References(pos, os.Stdout)
	case "hov":
		err = c.Hover(pos, os.Stdout)
	case "comp":
		err = c.Completion(pos, os.Stdout)
	}
	if err != nil {
		log.Fatalf("%v\n", err)
	}
}
