package protocol

import (
	"encoding/json"
	"strings"
)

// tsprotocol.go is not general enough to support other LSP servers besides
// gopls. Let's try to be compatible with all LSP servers.

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

// CodeActionLiteralSupport is a type alias that works around difficulty in initializing the pointer
// InitializeParams.Capabilities.TextDocument.CodeAction.CodeActionLiteralSupport.
type CodeActionLiteralSupport = struct {
	CodeActionKind struct {
		ValueSet []CodeActionKind `json:"valueSet"`
	} `json:"codeActionKind"`
}

func ToCodeActionOptions(v map[string]interface{}) (*CodeActionOptions, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var opt CodeActionOptions
	err = json.Unmarshal(b, &opt)
	if err != nil {
		return nil, err
	}
	return &opt, nil
}

// Locations is a type which represents the union of Location and []Location
type Locations []Location

func (ls *Locations) UnmarshalJSON(data []byte) error {
	d := strings.TrimSpace(string(data))
	if len(d) == 0 && strings.EqualFold(d, "null") {
		return nil
	}

	if d[0] == '[' {
		var locations []Location
		err := json.Unmarshal(data, &locations)
		if err != nil {
			return err
		}
		*ls = locations
	} else {
		var location Location
		err := json.Unmarshal(data, &location)
		if err != nil {
			return err
		}
		*ls = append(*ls, location)
	}

	return nil
}

// compItems is a type which represents the union of Location and []Location
type compList CompletionList

func (c *compList) UnmarshalJSON(data []byte) error {
	d := strings.TrimSpace(string(data))
	if len(d) == 0 && strings.EqualFold(d, "null") {
		return nil
	}

	if d[0] == '[' {
		var items []CompletionItem
		err := json.Unmarshal(data, &items)
		if err != nil {
			return err
		}

		c.Items = items
	} else {

		var tmp CompletionList
		err := json.Unmarshal(data, &tmp)
		if err != nil {
			return err
		}

		*c = compList(tmp)
	}

	return nil
}
