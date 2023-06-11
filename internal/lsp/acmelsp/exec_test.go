package acmelsp

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp"
	"github.com/fhs/acme-lsp/internal/lsp/acmelsp/config"
	"github.com/fhs/go-lsp-internal/lsp/protocol"
	"github.com/google/go-cmp/cmp"
)

func TestAbsDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dirs := []string{
		"/path/to/mod1",
		"./mod1",
		"../stuff/mod1",
	}
	got, err := AbsDirs(dirs)
	if err != nil {
		t.Fatalf("AbsDirs: %v", err)
	}
	want := []string{
		"/path/to/mod1",
		filepath.Join(cwd, "mod1"),
		filepath.Join(cwd, "../stuff/mod1"),
	}
	if !cmp.Equal(got, want) {
		t.Errorf("AbsDirs of %v is %v; want %v", dirs, got, want)
	}
}

func TestServerSetWorkspaces(t *testing.T) {
	cfg := &config.Config{
		File: config.File{
			RootDirectory:        "/",
			WorkspaceDirectories: []string{"/path/to/mod1", "/path/to/mod2"},
			Servers: map[string]*config.Server{
				"gopls": {
					Command: []string{"gopls"},
				},
			},
			FilenameHandlers: []config.FilenameHandler{
				{
					Pattern:   `\.go$`,
					ServerKey: "gopls",
				},
			},
		},
	}
	ss, err := NewServerSet(cfg, &mockDiagosticsWriter{ioutil.Discard})
	if err != nil {
		t.Fatalf("failed to create server set: %v", err)
	}
	defer ss.CloseAll()

	want, err := lsp.DirsToWorkspaceFolders(cfg.WorkspaceDirectories)
	if err != nil {
		t.Fatalf("DirsToWorkspaceFolders failed: %v", err)
	}
	got := ss.Workspaces()
	if !cmp.Equal(got, want) {
		t.Errorf("initial workspaces are %v; want %v", got, want)
	}

	added, err := lsp.DirsToWorkspaceFolders([]string{"/path/to/mod3"})
	if err != nil {
		t.Fatalf("DirsToWorkspaceFolders failed: %v", err)
	}
	want = append(want, added...)
	err = ss.DidChangeWorkspaceFolders(context.Background(), added, nil)
	if err != nil {
		t.Fatalf("ServerSet.AddWorkspaces: %v", err)
	}
	got = ss.Workspaces()
	if !cmp.Equal(got, want) {
		t.Errorf("after adding %v, workspaces are %v; want %v", added, got, want)
	}

	removed := want[:1]
	want = want[1:]
	err = ss.DidChangeWorkspaceFolders(context.Background(), nil, removed)
	if err != nil {
		t.Fatalf("ServerSet.RemoveWorkspaces: %v", err)
	}
	got = ss.Workspaces()
	if !cmp.Equal(got, want) {
		t.Errorf("after removing %v, workspaces are %v; want %v", removed, got, want)
	}
}

type mockDiagosticsWriter struct {
	io.Writer
}

func (dw *mockDiagosticsWriter) WriteDiagnostics(params *protocol.PublishDiagnosticsParams) {
	for _, diag := range params.Diagnostics {
		loc := &protocol.Location{
			URI:   params.URI,
			Range: diag.Range,
		}
		fmt.Fprintf(dw, "%v: %v\n", lsp.LocationLink(loc), diag.Message)
	}
}
