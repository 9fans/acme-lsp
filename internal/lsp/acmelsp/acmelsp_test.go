package acmelsp

import (
	"reflect"
	"testing"

	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

func TestPlumbLocation(t *testing.T) {
	loc := protocol.Location{
		URI: "file:///home/gopher/hello/main.go",
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      100,
				Character: 25,
			},
			End: protocol.Position{},
		},
	}
	want := plumb.Message{
		Src:  "acme-lsp",
		Dst:  "edit",
		Dir:  "/",
		Type: "text",
		Attr: &plumb.Attribute{
			Name:  "addr",
			Value: "101-#0+#25",
		},
		Data: []byte("/home/gopher/hello/main.go"),
	}
	got := plumbLocation(&loc)
	if reflect.DeepEqual(got, want) {
		t.Errorf("plumbLocation(%v) returned %v; exptected %v", loc, got, want)
	}
}
