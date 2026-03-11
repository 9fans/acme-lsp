package acmelsp

import (
	"sync"

	"9fans.net/acme-lsp/internal/lsp/acmelsp/config"
)

// HeadlessFileManager keeps track of open files in acme.
// It is used to synchronize text with LSP server.
//
// Note that we can't cache the *acmeutil.Win for the windows
// because having the ctl file open prevents del event from
// being delivered to acme/log file.
type HeadlessFileManager struct {
	ss   *ServerSet
	wins map[string]struct{} // set of open files
	mu   sync.Mutex

	cfg *config.Config
}

// NewHeadlessFileManager creates a new file manager, initialized with files currently open in acme.
func NewHeadlessFileManager(ss *ServerSet, cfg *config.Config) (*HeadlessFileManager, error) {
	fm := &HeadlessFileManager{
		ss:   ss,
		wins: make(map[string]struct{}),
		cfg:  cfg,
	}
	return fm, nil
}

// Run watches for files opened, closed, saved, or refreshed in acme
// and tells LSP server about it. It also formats files when it's saved.
func (fm *HeadlessFileManager) Run() {
	// We can potentially implement this using fsnotify,
	// but headless mode is only used for testing.
}

func (fm *HeadlessFileManager) DidChange(winid int) error {
	return nil
}
