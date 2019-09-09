package acmelsp

import (
	"fmt"
	"log"
	"sync"

	"9fans.net/go/acme"
	"github.com/fhs/acme-lsp/internal/acmeutil"
	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/text"
	"github.com/pkg/errors"
)

// ManageFiles watches for files opened, closed, saved, or refreshed in acme
// and tells LSP server about it. It also formats files when it's saved.
func ManageFiles(serverSet *lsp.ServerSet, fm *FileManager) {
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
			if err := fm.format(ev.ID, ev.Name); err != nil {
				log.Printf("Format failed in file manager: %v", err)
			}
		}
	}
}

// FileManager keeps track of open files in acme.
// It is used to synchronize text with LSP server.
//
// Note that we can't cache the *acmeutil.Win for the windows
// because having the ctl file open prevents del event from
// being delivered to acme/log file.
type FileManager struct {
	ss   *lsp.ServerSet
	wins map[string]struct{} // set of open files
	mu   sync.Mutex
}

// NewFileManager creates a new file manager, initialized with files currently open in acme.
func NewFileManager(ss *lsp.ServerSet) (*FileManager, error) {
	fm := &FileManager{
		ss:   ss,
		wins: make(map[string]struct{}),
	}

	wins, err := acme.Windows()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read list of acme index")
	}
	for _, info := range wins {
		err := fm.didOpen(info.ID, info.Name)
		if err != nil {
			return nil, err
		}
	}
	return fm, nil
}

func (fm *FileManager) withClient(winid int, name string, f func(*lsp.Conn, *acmeutil.Win) error) error {
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
	return f(s.Conn, win)
}

func (fm *FileManager) didOpen(winid int, name string) error {
	return fm.withClient(winid, name, func(c *lsp.Conn, w *acmeutil.Win) error {
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
		return c.DidOpen(name, b)
	})
}

func (fm *FileManager) didClose(name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	delete(fm.wins, name)

	return fm.withClient(-1, name, func(c *lsp.Conn, _ *acmeutil.Win) error {
		return c.DidClose(name)
	})
}

func (fm *FileManager) didChange(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *lsp.Conn, w *acmeutil.Win) error {
		b, err := w.ReadAll("body")
		if err != nil {
			return err
		}
		return c.DidChange(name, b)
	})
}

func (fm *FileManager) didSave(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *lsp.Conn, w *acmeutil.Win) error {
		b, err := w.ReadAll("body")
		if err != nil {
			return err
		}

		// TODO(fhs): Maybe DidChange is not needed with includeText option to DidSave?
		err = c.DidChange(name, b)
		if err != nil {
			return err
		}
		return c.DidSave(name)
	})
}

func (fm *FileManager) format(winid int, name string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, ok := fm.wins[name]; !ok {
		return nil // Unknown language server.
	}
	return fm.withClient(winid, name, func(c *lsp.Conn, w *acmeutil.Win) error {
		return FormatFile(c, text.ToURI(name), w)
	})
}
