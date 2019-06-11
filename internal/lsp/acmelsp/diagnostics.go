package acmelsp

import (
	"fmt"
	"sync"

	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

// diagWin implements client.DiagnosticsWriter.
// It writes diagnostics to an acme window.
// It will create the diagnostics window on-demand, recreating it if necessary.
type diagWin struct {
	name string
	*acmeutil.Win
	dead bool
	mu   sync.Mutex
}

func newDiagWin(name string) *diagWin {
	return &diagWin{
		name: name,
		dead: true,
	}
}

func (dw *diagWin) restart() error {
	if !dw.dead {
		return nil
	}
	w, err := acmeutil.Hijack(dw.name)
	if err != nil {
		w, err = acmeutil.NewWin()
	}
	if err != nil {
		return err
	}
	w.Name(dw.name)
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
				if string(ev.Text) == "Del" {
					return
				}
			}
			dw.WriteEvent(ev)
		}
	}()
	return nil
}

func (dw *diagWin) WriteDiagnostics(diags map[protocol.DocumentURI][]protocol.Diagnostic) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

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
			fmt.Fprintf(body, "%v: %v\n", client.LocationLink(loc), diag.Message)
		}
	}
	return dw.Ctl("clean")
}

func NewDiagnosticsWriter() client.DiagnosticsWriter {
	return newDiagWin("/LSP/Diagnostics")
}
