package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"9fans.net/go/acme"
	lsp "github.com/sourcegraph/go-lsp"
)

func watchLog(ch chan<- *acme.LogEvent) {
	alog, err := acme.Log()
	if err != nil {
		panic(err)
	}
	defer alog.Close()
	for {
		ev, err := alog.Read()
		if err != nil {
			panic(err)
		}
		ch <- &ev
	}
}

type focusWin struct {
	lang string
	id   int
	q0   int
	pos  *lsp.TextDocumentPositionParams
	name string
	w    *win
	mu   sync.Mutex
}

func (fw *focusWin) Reset() {
	fw.lang = ""
	fw.id = -1
	fw.q0 = -1
	fw.pos = nil
	fw.name = ""
	if fw.w != nil {
		fw.w.CloseFiles()
		fw.w = nil
	}
}

func (fw *focusWin) Update() bool {
	w, err := openWin(fw.id)
	if err != nil {
		return false
	}
	q0, _, err := w.ReadDotAddr()
	if err != nil {
		return false
	}
	if q0 == fw.q0 {
		return false
	}
	pos, name, err := getAcmeWinPos(fw.id)
	if err != nil {
		return false
	}
	fw.q0 = q0
	fw.pos = pos
	fw.name = name
	fw.w = w
	return true
}

type outputWin struct {
	*win
	body  io.Writer
	event <-chan *acme.Event
}

func newOutputWin() (*outputWin, error) {
	w, err := newWin()
	if err != nil {
		return nil, err
	}
	w.Name("/Lsp/watch")
	return &outputWin{
		win:   w,
		body:  w.FileReadWriter("body"),
		event: w.EventChan(),
	}, nil
}

func (w *outputWin) Close() {
	w.Del(true)
	w.CloseFiles()
}

func (w *outputWin) Update(fw *focusWin, c *lspClient) {
	b, err := fw.w.ReadAll("body")
	if err != nil {
		log.Printf("failed to read source body: %v\n", err)
		return
	}
	err = c.DidOpen(fw.name, b)
	if err != nil {
		log.Printf("DidOpen failed: %v\n", err)
		return
	}
	defer func() {
		err = c.DidClose(fw.name)
		if err != nil {
			log.Printf("DidClose failed: %v\n", err)
		}
	}()

	w.Clear()
	/*
		err = c.Completion(fw.pos, w.body)
		if err != nil {
			log.Printf("Completion failed: %v\n", err)
		}
		fmt.Fprintf(w.body, "---\n")
		err = c.SignatureHelp(fw.pos, w.body)
		if err != nil {
			log.Printf("SignatureHelp failed: %v\n", err)
		}
		fmt.Fprintf(w.body, "---\n")
	*/
	err = c.Hover(fw.pos, w.body)
	if err != nil {
		log.Printf("Hover failed: %v\n", err)
	}
	w.Ctl("clean")
}

func (c *lspClient) Watch() {
	w, err := newOutputWin()
	if err != nil {
		log.Fatalf("failed to create acme window: %v\n", err)
	}
	defer w.Close()

	var fw focusWin
	fw.Reset()

	logch := make(chan *acme.LogEvent, 0)
	go watchLog(logch)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case ev := <-logch:
			//fmt.Printf("event: %v\n", ev)
			fw.mu.Lock()
			if ev.Op == "focus" && strings.HasSuffix(ev.Name, ".go") {
				fw.lang = "go"
				fw.id = ev.ID
			} else {
				fw.Reset()
			}
			fw.mu.Unlock()

		case <-ticker.C:
			fw.mu.Lock()
			if fw.lang == "go" && fw.Update() {
				fmt.Printf("Watch: id=%v q0=%v\n", fw.id, fw.q0)
				fmt.Printf("Watch: pos=%v\n", fw.pos)
				w.Update(&fw, c)
			}
			fw.mu.Unlock()

		case ev := <-w.event:
			if ev.C1 == 'M' && ev.C2 == 'x' && string(ev.Text) == "Del" {
				w.Close()
				break loop
			}
			w.WriteEvent(ev)
		}
	}
}
