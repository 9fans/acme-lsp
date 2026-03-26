package text

import (
	"runtime"
	"testing"

	"9fans.net/internal/go-lsp/lsp/protocol"
)

func TestToURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	for _, tc := range []struct {
		name string
		uri  protocol.DocumentURI
	}{
		{"/home/gopher/hello.go", "file:///home/gopher/hello.go"},
		{"/home/タロ/src/hello.go", "file:///home/%E3%82%BF%E3%83%AD/src/hello.go"},
		{"/usr/include/c++/v1/deque", "file:///usr/include/c++/v1/deque"},
	} {
		uri := ToURI(tc.name)
		if uri != tc.uri {
			t.Errorf("ToURI(%q) is %q; expected %q", tc.name, uri, tc.uri)
		}
	}
}

func TestToPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: failing on windows due to file path issues")
	}

	for _, tc := range []struct {
		name string
		uri  protocol.DocumentURI
	}{
		{"/home/gopher/hello.go", "/home/gopher/hello.go"},
		{"/home/gopher/hello.go", "file:///home/gopher/hello.go"},
		{"/home/タロ/src/hello.go", "file:///home/%E3%82%BF%E3%83%AD/src/hello.go"},
		{"/usr/include/c++/v1/deque", "file:///usr/include/c%2B%2B/v1/deque"},
	} {
		name := ToPath(tc.uri)
		if name != tc.name {
			t.Errorf("ToPath(%q) is %q; expected %q", tc.uri, name, tc.name)
		}
	}
}
