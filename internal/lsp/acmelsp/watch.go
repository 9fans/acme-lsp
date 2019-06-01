package acmelsp

import (
	"io"
	"log"
	"sync"
	"time"

	"9fans.net/go/acme"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp/text"
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
	pos  *protocol.TextDocumentPositionParams
	name string
	w    *acmeutil.Win
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
	w, err := acmeutil.OpenWin(fw.id)
	if err != nil {
		return false
	}
	q0, _, err := w.CurrentAddr()
	if err != nil {
		return false
	}
	if q0 == fw.q0 {
		return false
	}
	pos, name, err := text.Position(w)
	if err != nil {
		return false
	}
	fw.q0 = q0
	fw.pos = pos
	fw.name = name
	fw.w = w
	return true
}

func notifyPosChange(serverSet *client.ServerSet, ch chan<- *focusWin) {
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
			if serverSet.MatchFile(ev.Name) != nil && ev.Op == "focus" {
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
	*acmeutil.Win
	body  io.Writer
	event <-chan *acme.Event
}

func newOutputWin() (*outputWin, error) {
	w, err := acmeutil.NewWin()
	if err != nil {
		return nil, err
	}
	w.Name("/Lsp/watch")
	return &outputWin{
		Win:   w,
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

// Watch creates an acme window where output of cmd is written after each
// cursor position change in acme. Cmd is either 'comp', 'sig', or 'hov',
// for completion, signature, and hover respectively.
func Watch(serverSet *client.ServerSet, cmd string) {
	w, err := newOutputWin()
	if err != nil {
		log.Fatalf("failed to create acme window: %v\n", err)
	}
	defer w.Close()

	fch := make(chan *focusWin)
	go notifyPosChange(serverSet, fch)

loop:
	for {
		select {
		case fw := <-fch:
			fw.mu.Lock()
			s, found, err := serverSet.StartForFile(fw.name)
			if err != nil {
				log.Printf("failed to start language server: %v\n", err)
			}
			if found {
				w.Update(fw, s.Conn, cmd)
			}
			fw.mu.Unlock()

		case ev := <-w.event:
			if ev == nil {
				break loop
			}
			switch ev.C2 {
			case 'x', 'X': // execute
				if string(ev.Text) == "Del" {
					break loop
				}
			}
			w.WriteEvent(ev)
		}
	}
}
