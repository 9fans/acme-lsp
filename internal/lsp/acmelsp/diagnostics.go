package acmelsp

import (
	"fmt"
	"sync"
	"time"

	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/go-lsp-internal/lsp/protocol"
)

// diagWin implements client.DiagnosticsWriter.
// It writes diagnostics to an acme window.
// It will create the diagnostics window on-demand, recreating it if necessary.
type diagWin struct {
	name string // window name
	*acmeutil.Win
	paramsChan chan *protocol.PublishDiagnosticsParams
	updateChan chan struct{}

	dead bool // window has been closed
	mu   sync.Mutex
}

func newDiagWin(name string) *diagWin {
	return &diagWin{
		name:       name,
		updateChan: make(chan struct{}),
		paramsChan: make(chan *protocol.PublishDiagnosticsParams, 100),
		dead:       true,
	}
}

func (dw *diagWin) restart() error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	if !dw.dead {
		return nil
	}
	w, err := acmeutil.Hijack(dw.name)
	if err != nil {
		w, err = acmeutil.NewWin()
		if err != nil {
			return err
		}
		w.Name(dw.name)
		w.Write("tag", []byte("Reload "))
	}
	dw.Win = w
	dw.dead = false

	go func() {
		defer func() {
			dw.mu.Lock()
			dw.Del(true)
			dw.CloseFiles()
			dw.dead = true
			dw.mu.Unlock()
		}()

		for ev := range dw.EventChan() {
			if ev == nil {
				return
			}
			switch ev.C2 {
			case 'x', 'X': // execute
				switch string(ev.Text) {
				case "Del":
					return
				case "Reload":
					dw.updateChan <- struct{}{}
					continue
				}
			}
			dw.WriteEvent(ev)
		}
	}()
	return nil
}

func (dw *diagWin) update(diags map[protocol.DocumentURI][]protocol.Diagnostic) error {
	if err := dw.restart(); err != nil {
		return err
	}

	dw.Clear()
	body := dw.FileReadWriter("body")
	for uri, uriDiag := range diags {
		for _, diag := range uriDiag {
			loc := &protocol.Location{
				URI:   uri,
				Range: diag.Range,
			}
			fmt.Fprintf(body, "%v: %v\n", lsp.LocationLink(loc), diag.Message)
		}
	}
	return dw.Ctl("clean")
}

func (dw *diagWin) WriteDiagnostics(params *protocol.PublishDiagnosticsParams) {
	dw.paramsChan <- params
}

func NewDiagnosticsWriter() DiagnosticsWriter {
	dw := newDiagWin("/LSP/Diagnostics")

	// Collect stream of diagnostics updates and write them all
	// after certain interval if they need to be updated.
	go func() {
		diags := make(map[protocol.DocumentURI][]protocol.Diagnostic)
		ticker := time.NewTicker(time.Second)
		needsUpdate := false
		for {
			select {
			case <-ticker.C:
				if needsUpdate {
					dw.update(diags)
					needsUpdate = false
				}

			case <-dw.updateChan: // user request
				dw.update(diags)
				needsUpdate = false

			case params := <-dw.paramsChan:
				if len(diags[params.URI]) == 0 && len(params.Diagnostics) == 0 {
					continue
				}
				diags[params.URI] = params.Diagnostics
				needsUpdate = true
			}
		}
	}()
	return dw
}
