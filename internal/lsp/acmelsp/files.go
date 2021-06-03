package acmelsp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/fhs/acme-lsp/internal/acme"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/golang_org_x_tools/lsp/protocol"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/acme-lsp/internal/lsp/text"
)

// FileManager keeps track of open files in acme.
// It is used to synchronize text with LSP server.
//
// Note that we can't cache the *acmeutil.Win for the windows
// because having the ctl file open prevents del event from
// being delivered to acme/log file.
type FileManager struct {
	ss   *ServerSet
	wins map[string]struct{} // set of open files
	mu   sync.Mutex

	cfg *config.Config
}

// NewFileManager creates a new file manager, initialized with files currently open in acme.
func NewFileManager(ss *ServerSet, cfg *config.Config) (*FileManager, error) {
	fm := &FileManager{
		ss:   ss,
		wins: make(map[string]struct{}),
		cfg:  cfg,
	}

	wins, err := acme.Windows()
	if err != nil {
		return nil, fmt.Errorf("failed to read list of acme index: %v", err)
	}
	for _, info := range wins {
		err := fm.didOpen(info.ID, info.Name)
		if err != nil {
			return nil, err
		}
	}
	return fm, nil
}

// Run watches for files opened, closed, saved, or refreshed in acme
// and tells LSP server about it. It also formats files when it's saved.
func (fm *FileManager) Run() {
	alog, err := acme.Log()
	if err != nil {
		log.Printf("file manager opening acme/log: %v", err)
		return
	}
	defer alog.Close()

	for {
		ev, err := alog.Read()
		if err != nil {
			log.Printf("file manager reading acme/log: %v", err)
			return
		}
		switch ev.Op {
		case "new":
			if err := fm.didOpen(ev.ID, ev.Name); err != nil {
				log.Printf("didOpen failed in file manager: %v", err)
			}
		case "del":
			if err := fm.didClose(ev.Name); err != nil {
				log.Printf("didClose failed in file manager: %v", err)
			}
		case "get":
			if err := fm.didChange(ev.ID, ev.Name); err != nil {
				log.Printf("didChange failed in file manager: %v", err)
			}
		case "put":
			if err := fm.didSave(ev.ID, ev.Name); err != nil {
				log.Printf("didSave failed in file manager: %v", err)
			}
			if fm.cfg.FormatOnPut {
				if err := fm.format(ev.ID, ev.Name); err != nil && Verbose {
					log.Printf("Format failed in file manager: %v", err)
				}
			}
		}
	}
}

func (fm *FileManager) withClient(winid int, name string, f func(*Client, *acmeutil.Win) error) error {
	s, found, err := fm.ss.StartForFile(name)
	if err != nil {
		return err
	}
	if !found {
		return nil // Unknown language server.
	}

	var win *acmeutil.Win
	if winid >= 0 {
		w, err := acmeutil.OpenWin(winid)
		if err != nil {
			return err
		}
		defer w.CloseFiles()
		win = w
	}
	return f(s.Client, win)
}

func (fm *FileManager) didOpen(winid int, name string) error {
	return fm.withClient(winid, name, func(c *Client, w *acmeutil.Win) error {
		fm.mu.Lock()
		defer fm.mu.Unlock()

		if _, ok := fm.wins[name]; ok {
			return fmt.Errorf("file already open in file manager: %v", name)
		}
		fm.wins[name] = struct{}{}

		b, err := w.ReadAll("body")
		if err != nil {
			return err
		}
		return lsp.DidOpen(context.Background(), c, name, c.cfg.FilenameHandler.LanguageID, b)
	})
}

func (fm *FileManager) didClose(name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	delete(fm.wins, name)

	return fm.withClient(-1, name, func(c *Client, _ *acmeutil.Win) error {
		return lsp.DidClose(context.Background(), c, name)
	})
}

func (fm *FileManager) didChange(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *Client, w *acmeutil.Win) error {
		b, err := w.ReadAll("body")
		if err != nil {
			return err
		}
		return lsp.DidChange(context.Background(), c, name, b)
	})
}

func (fm *FileManager) DidChange(winid int) error {
	w, err := acmeutil.OpenWin(winid)
	if err != nil {
		return err
	}
	defer w.CloseFiles()

	name, err := w.Filename()
	if err != nil {
		return fmt.Errorf("could not get filename for window %v: %v", winid, err)
	}
	// TODO(fhs): we are opening the window again in didChange.
	return fm.didChange(winid, name)
}

func (fm *FileManager) didSave(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *Client, w *acmeutil.Win) error {
		b, err := w.ReadAll("body")
		if err != nil {
			return err
		}

		// TODO(fhs): Maybe DidChange is not needed with includeText option to DidSave?
		err = lsp.DidChange(context.Background(), c, name, b)
		if err != nil {
			return err
		}
		return lsp.DidSave(context.Background(), c, name)
	})
}

func (fm *FileManager) format(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *Client, w *acmeutil.Win) error {
		doc := &protocol.TextDocumentIdentifier{
			URI: text.ToURI(name),
		}
		return CodeActionAndFormat(context.Background(), c, doc, w, fm.cfg.CodeActionsOnPut)
	})
}
