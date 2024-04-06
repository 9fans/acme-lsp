package acmelsp

import (
	"flag"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/fhs/9fans-go/plumb"
	"9fans.net/acme-lsp/internal/lsp"
	"9fans.net/acme-lsp/internal/lsp/acmelsp/config"
	"9fans.net/internal/go-lsp/lsp/protocol"
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
		name       string
		args       []string
		debug      bool
		serverInfo []*ServerInfo
		workspaces []string
		err        string
	}{
		{"Debug", []string{"-debug"}, true, nil, nil, ""},
		{
			"OneWorkspace",
			[]string{"-workspaces", "/path/to/mod1"},
			false,
			nil,
			[]string{"/path/to/mod1"},
			"",
		},
		{
			"TwoWorkspaces",
			[]string{"-workspaces", "/go/mod1:/go/mod2"},
			false,
			nil,
			[]string{"/go/mod1", "/go/mod2"},
			"",
		},
		{
			"ServerFlag",
			[]string{"-server", `\.go$:gopls -rpc.trace`},
			false,
			[]*ServerInfo{
				{
					Server: &config.Server{
						Command: []string{"gopls", "-rpc.trace"},
					},
					Re: regexp.MustCompile(`\.go$`),
				},
			},
			nil,
			"",
		},
		{
			"DialFlag",
			[]string{"-dial", `\.go$:localhost:4389`},
			false,
			[]*ServerInfo{
				{
					Server: &config.Server{
						Address: "localhost:4389",
					},
					Re: regexp.MustCompile(`\.go$`),
				},
			},
			nil,
			"",
		},
		{
			"BadServerFlag",
			[]string{"-server", `gopls -rpc.trace`},
			false,
			nil,
			nil,
			"flag value must contain a colon",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			f := flag.NewFlagSet("acme-lsp", flag.ContinueOnError)
			f.SetOutput(ioutil.Discard)

			cfg := config.Default()
			err := cfg.ParseFlags(config.LangServerFlags, f, tc.args)
			if len(tc.err) > 0 {
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("for %q, error %q does not contain %q", tc.args, err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}
			ss, err := NewServerSet(cfg, NewDiagnosticsWriter())
			if err != nil {
				t.Fatalf("ParseFlagSet failed: %v", err)
			}
			if cfg.Verbose != tc.debug {
				t.Errorf("-debug flag didn't turn on debugging")
			}
			got := ss.Workspaces()
			want, err := lsp.DirsToWorkspaceFolders(tc.workspaces)
			if err != nil {
				t.Fatalf("DirsToWorkspaceFolders failed: %v", err)
			}
			if !cmp.Equal(got, want) {
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
				if got, want := got.Command, want.Command; !cmp.Equal(got, want) {
					t.Errorf("lsp server args for %v is %v; want %v", tc.args, got, want)
				}
				if got, want := got.Address, want.Address; !cmp.Equal(got, want) {
					t.Errorf("lsp server dial address for %v is %v; want %v", tc.args, got, want)
				}
			}
		})
	}
}
