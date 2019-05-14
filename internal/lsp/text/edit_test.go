package text

import (
	"testing"

	"github.com/fhs/acme-lsp/internal/lsp/protocol"
)

func TestURI(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  protocol.DocumentURI
	}{
		{"/home/gopher/hello.go", "file:///home/gopher/hello.go"},
	} {
		uri := ToURI(tc.name)
		if uri != tc.uri {
			t.Errorf("ToURI(%q) is %q; expected %q", tc.name, uri, tc.uri)
		}
		name := ToPath(tc.uri)
		if name != tc.name {
			t.Errorf("ToPath(%q) is %q; expected %q", tc.uri, name, tc.name)
		}
	}
}
