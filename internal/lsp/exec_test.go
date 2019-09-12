package lsp

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/google/go-cmp/cmp"
)

func TestAbsDirs(t *testing.T) {
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
	ss := NewServerSet(&Config{
		DiagWriter: &mockDiagosticsWriter{ioutil.Discard},
		RootDir:    "/",
		Workspaces: nil,
	})
	err := ss.Register(`\.go$`, []string{"gopls"})
	if err != nil {
		t.Fatalf("ServerSet.Register: %v", err)
	}
	defer ss.CloseAll()

	got := ss.Workspaces()
	if len(got) > 0 {
		t.Errorf("default workspaces are %v; want empty slice", got)
	}

	want := []string{"/path/to/mod1", "/path/to/mod2"}
	err = ss.InitWorkspaces(want)
	if err != nil {
		t.Fatalf("ServerSet.InitWorkspaces: %v", err)
	}
	got = ss.Workspaces()
	if !cmp.Equal(got, want) {
		t.Errorf("initial workspaces are %v; want %v", got, want)
	}

	added := "/path/to/mod3"
	want = append(want, added)
	err = ss.AddWorkspaces([]string{added})
	if err != nil {
		t.Fatalf("ServerSet.AddWorkspaces: %v", err)
	}
	got = ss.Workspaces()
	if !cmp.Equal(got, want) {
		t.Errorf("after adding %v, workspaces are %v; want %v", added, got, want)
	}

	removed := want[0]
	want = want[1:]
	err = ss.RemoveWorkspaces([]string{removed})
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

func (dw *mockDiagosticsWriter) WriteDiagnostics(diags map[protocol.DocumentURI][]protocol.Diagnostic) error {
	for uri, uriDiag := range diags {
		for _, diag := range uriDiag {
			loc := &protocol.Location{
				URI:   uri,
				Range: diag.Range,
			}
			fmt.Fprintf(dw, "%v: %v\n", LocationLink(loc), diag.Message)
		}
	}
	return nil
}
