package text

import (
	"runtime"
	"testing"

	"9fans.net/internal/go-lsp/lsp/protocol"
)

func TestURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

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
