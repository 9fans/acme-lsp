package acmelsp

import (
	"io"
	"log"
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

// focusWindow represents the last focused window.
//
// Note that we can't cache the *acmeutil.Win for the window
// because having the ctl file open prevents del event from
// being delivered to acme/log file.
type focusWin struct {
	id   int
	q0   int
	name string
}

func newFocusWin() *focusWin {
	var fw focusWin
	fw.Reset()
	return &fw
}

func (fw *focusWin) Reset() {
	fw.id = -1
	fw.q0 = -1
	fw.name = ""
}

func (fw *focusWin) SetQ0() bool {
	if fw.id < 0 {
		return false
	}
	w, err := acmeutil.OpenWin(fw.id)
	if err != nil {
		return false
	}
	defer w.CloseFiles()

	q0, _, err := w.CurrentAddr()
	if err != nil {
		return false
	}
	fw.q0 = q0
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
			if serverSet.MatchFile(ev.Name) != nil && ev.Op == "focus" {
				// TODO(fhs): we should really make use of context
				// and cancel outstanding rpc requests on previously focused window.
				fw.id = ev.ID
				fw.name = ev.Name
			} else {
				fw.Reset()
			}

		case <-ticker.C:
			if fw.SetQ0() && pos[fw.id] != fw.q0 {
				pos[fw.id] = fw.q0
				ch <- &focusWin{ // send a copy
					id:   fw.id,
					q0:   fw.q0,
					name: fw.name,
				}
			}
		}
	}
}

type outputWin struct {
	*acmeutil.Win
	body  io.Writer
	event <-chan *acme.Event
	fm    *FileManager
}

func newOutputWin(fm *FileManager, name string) (*outputWin, error) {
	w, err := acmeutil.NewWin()
	if err != nil {
		return nil, err
	}
	w.Name(name)
	return &outputWin{
		Win:   w,
		body:  w.FileReadWriter("body"),
		event: w.EventChan(),
		fm:    fm,
	}, nil
}

func (w *outputWin) Close() {
	w.Del(true)
	w.CloseFiles()
}

func winPosition(id int) (*protocol.TextDocumentPositionParams, string, error) {
	w, err := acmeutil.OpenWin(id)
	if err != nil {
		return nil, "", err
	}
	defer w.CloseFiles()

	return text.Position(w)
}

// Update writes result of cmd to output window.
func (w *outputWin) Update(fw *focusWin, c *client.Conn, cmd string) {
	pos, _, err := winPosition(fw.id)
	if err != nil {
		log.Printf("failed to get window position: %v\n", err)
		return
	}

	// Assume file is already opened by file management.
	err = w.fm.didChange(fw.id, fw.name)
	if err != nil {
		log.Printf("DidChange failed: %v\n", err)
		return
	}

	w.Clear()
	switch cmd {
	case "comp":
		err = c.Completion(pos, w.body)
		if err != nil {
			log.Printf("Completion failed: %v\n", err)
		}

	case "sig":
		err = c.SignatureHelp(pos, w.body)
		if err != nil {
			log.Printf("SignatureHelp failed: %v\n", err)
		}
	case "hov":
		err = c.Hover(pos, w.body)
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
func Watch(serverSet *client.ServerSet, fm *FileManager, cmd string) {
	w, err := newOutputWin(fm, "/LSP/win/"+cmd)
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
			s, found, err := serverSet.StartForFile(fw.name)
			if err != nil {
				log.Printf("failed to start language server: %v\n", err)
			}
			if found {
				w.Update(fw, s.Conn, cmd)
			}

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
