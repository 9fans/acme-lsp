package main

import (
	"io"
	"log"
	"sync"
	"time"

	"9fans.net/go/acme"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/client"
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
	id   int
	q0   int
	pos  *lsp.TextDocumentPositionParams
	name string
	w    *win
	mu   sync.Mutex
}

func newFocusWin() *focusWin {
	var fw focusWin
	fw.Reset()
	return &fw
}

func (fw *focusWin) Reset() {
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
	pos, name, err := w.Position()
	if err != nil {
		return false
	}
	fw.q0 = q0
	fw.pos = pos
	fw.name = name
	fw.w = w
	return true
}

func notifyPosChange(ch chan<- *focusWin) {
	fw := newFocusWin()
	logch := make(chan *acme.LogEvent)
	go watchLog(logch)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	pos := make(map[int]int) // winid -> q0

	for {
		select {
		case ev := <-logch:
			fw.mu.Lock()
			si := findServer(ev.Name)
			if si != nil && ev.Op == "focus" {
				fw.id = ev.ID
			} else {
				fw.Reset()
			}
			fw.mu.Unlock()

		case <-ticker.C:
			fw.mu.Lock()
			if fw.Update() && pos[fw.id] != fw.q0 {
				pos[fw.id] = fw.q0
				ch <- fw
			}
			fw.mu.Unlock()
		}
	}
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

func (w *outputWin) Update(fw *focusWin, c *client.Conn, cmd string) {
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
	switch cmd {
	case "comp":
		err = c.Completion(fw.pos, w.body)
		if err != nil {
			log.Printf("Completion failed: %v\n", err)
		}

	case "sig":
		err = c.SignatureHelp(fw.pos, w.body)
		if err != nil {
			log.Printf("SignatureHelp failed: %v\n", err)
		}
	case "hov":
		err = c.Hover(fw.pos, w.body)
		if err != nil {
			log.Printf("Hover failed: %v\n", err)
		}
	default:
		log.Fatalf("invalid command %q\n", cmd)
	}
	w.Ctl("clean")
}

func watch(cmd string) {
	w, err := newOutputWin()
	if err != nil {
		log.Fatalf("failed to create acme window: %v\n", err)
	}
	defer w.Close()

	fch := make(chan *focusWin)
	go notifyPosChange(fch)

loop:
	for {
		select {
		case fw := <-fch:
			fw.mu.Lock()
			s, err := startServerForFile(fw.name)
			if err != nil {
				log.Printf("failed to start language server: %v\n", err)
			} else {
				w.Update(fw, s.Conn, cmd)
			}
			fw.mu.Unlock()

		case ev := <-w.event:
			if ev.C1 == 'M' && ev.C2 == 'x' && string(ev.Text) == "Del" {
				break loop
			}
			w.WriteEvent(ev)
		}
	}
}
