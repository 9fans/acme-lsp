package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// tsprotocol.go (copied from golang.org/x/tools/internal/lsp/protocol)
// is not general enough to support other LSP servers besides gopls. Let's
// try to be compatible with all LSP servers.

func (m *MarkupContent) UnmarshalJSON(data []byte) error {
	d := strings.TrimSpace(string(data))
	if len(d) == 0 {
		return nil
	}
	switch d[0] {
	case '"': // string (deprecated in LSP)
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		m.Kind = PlainText
		m.Value = s
		return nil

	case '[': // []MarkedString (deprecated in LSP)
		var mslist []MarkupContent
		err := json.Unmarshal(data, &mslist)
		if err != nil {
			return err
		}
		var b strings.Builder
		for i, ms := range mslist {
			if i > 0 {
				b.WriteRune('\n')
			}
			b.WriteString(ms.Value)
		}
		m.Kind = PlainText
		m.Value = b.String()
		return nil
	}

	// MarkedString (deprecated in LSP) or MarkedContent
	type noUnmarshal MarkupContent
	m.Kind = PlainText // for MarkedString
	return json.Unmarshal(data, (*noUnmarshal)(m))
}

func (mt MessageType) String() string {
	switch mt {
	case Error:
		return "Error"
	case Warning:
		return "Warning"
	case Info:
		return "Info"
	case Log:
		return "Log"
	}
	return fmt.Sprintf("MessageType(%v)", int(mt))
}

type InitializeResult1 struct {
	InitializeResult
	Capabilities ServerCapabilities1 `json:"capabilities"`
}

type ServerCapabilities1 struct {
	ServerCapabilities
	Workspace *struct {
		WorkspaceFolders *struct {
			Supported           bool                `json:"supported,omitempty"`
			ChangeNotifications ChangeNotifications `json:"changeNotifications,omitempty"` // string | boolean
		} `json:"workspaceFolders,omitempty"`
	} `json:"workspace,omitempty"`
}

// ChangeNotifications contains either a bool or a string value.
type ChangeNotifications struct {
	Value interface{}
}

// UnmarshalJSON unmarshals JSON data into ChangeNotifications.
func (cn *ChangeNotifications) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		cn.Value = b
		return nil
	}

	var s string
	err := json.Unmarshal(data, &s)
	if err == nil {
		cn.Value = s
	}
	return err
}

// CodeActionLiteralSupport is a type alias that works around difficulty in initializing the pointer
// InitializeParams.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.
type CodeActionLiteralSupport = struct {
	CodeActionKind struct {
		ValueSet []CodeActionKind `json:"valueSet"`
	} `json:"codeActionKind"`
}
