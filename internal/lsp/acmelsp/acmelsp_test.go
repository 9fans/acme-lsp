package acmelsp

import (
	"flag"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"9fans.net/go/plumb"
	"github.com/fhs/acme-lsp/internal/lsp/client"
	"github.com/fhs/acme-lsp/internal/lsp/protocol"
	"github.com/google/go-cmp/cmp"
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

func TestParseFlagSet(t *testing.T) {
	tt := []struct {
		args       []string
		debug      bool
		serverInfo []*client.ServerInfo
		workspaces []string
		err        string
	}{
		{[]string{"-debug"}, true, nil, nil, ""},
		{
			[]string{"-workspaces", "/path/to/mod1"},
			false,
			nil,
			[]string{"/path/to/mod1"},
			"",
		},
		{
			[]string{"-workspaces", "/go/mod1:/go/mod2"},
			false,
			nil,
			[]string{"/go/mod1", "/go/mod2"},
			"",
		},
		{
			[]string{"-server", `\.go$:gopls -rpc.trace`},
			false,
			[]*client.ServerInfo{
				{
					Re:   regexp.MustCompile(`\.go$`),
					Args: []string{"gopls", "-rpc.trace"},
				},
			},
			nil,
			"",
		},
		{
			[]string{"-dial", `\.go$:localhost:4389`},
			false,
			[]*client.ServerInfo{
				{
					Re:   regexp.MustCompile(`\.go$`),
					Addr: "localhost:4389",
				},
			},
			nil,
			"",
		},
		{
			[]string{"-server", `gopls -rpc.trace`},
			false,
			nil,
			nil,
			"flag value must contain a colon",
		},
	}

	for _, tc := range tt {
		f := flag.NewFlagSet("acme-lsp", flag.ContinueOnError)
		f.SetOutput(ioutil.Discard)

		ss, debug, err := ParseFlagSet(f, tc.args, nil)
		if len(tc.err) > 0 {
			if !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("for %q, error %q does not contain %q", tc.args, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseFlagSet failed: %v", err)
		}
		if debug != tc.debug {
			t.Errorf("-debug flag didn't turn on debugging")
		}
		if got, want := ss.Workspaces(), tc.workspaces; !cmp.Equal(got, want) {
			t.Errorf("workspaces are %v; want %v", got, want)
		}
		if len(tc.serverInfo) > 0 {
			if got, want := len(ss.Data), len(tc.serverInfo); got != want {
				t.Fatalf("%v servers registered for %v; want %v", got, tc.args, want)
			}
			got := tc.serverInfo[0]
			want := ss.Data[0]
			if got, want := got.Re.String(), want.Re.String(); got != want {
				t.Errorf("filename pattern for %v is %v; want %v", got, tc.args, want)
			}
			if got, want := got.Args, want.Args; !cmp.Equal(got, want) {
				t.Errorf("lsp server args for %v is %v; want %v", tc.args, got, want)
			}
			if got, want := got.Addr, want.Addr; !cmp.Equal(got, want) {
				t.Errorf("lsp server dial address for %v is %v; want %v", tc.args, got, want)
			}
		}
	}
}
