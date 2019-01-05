// Package lsp defines some structs for Language Server Protocol.
//
// Currently, we only redefine some structs here because of an upstream bug:
// https://github.com/sourcegraph/go-lsp/issues/2
//
package lsp

import (
	"encoding/json"
	"strings"

	lsp "github.com/sourcegraph/go-lsp"
)

type Hover struct {
	Contents MarkedStringList `json:"contents"`
	Range    *lsp.Range       `json:"range,omitempty"`
}

type MarkedString struct {
	Language string `json:"language"`
	Value    string `json:"value"`
}

func (m *MarkedString) UnmarshalJSON(data []byte) error {
	if d := strings.TrimSpace(string(data)); len(d) > 0 && d[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		m.Value = s
		return nil
	}
	type noUnmarshal MarkedString
	ms := (*noUnmarshal)(m)
	return json.Unmarshal(data, ms)
}

type MarkedStringList []MarkedString

func (r *MarkedStringList) UnmarshalJSON(data []byte) error {
	if d := strings.TrimSpace(string(data)); len(d) > 0 && d[0] == '[' {
		type noUnmarshal MarkedStringList
		return json.Unmarshal(data, (*noUnmarshal)(r))
	}
	*r = make(MarkedStringList, 1)
	return json.Unmarshal(data, &(*r)[0])
}

type ShowMessageParams struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}

type LogMessageParams struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}

type MessageType int

const (
	MTError MessageType = 1 + iota
	MTWarning
	Info
	Log
)

func (mt MessageType) String() string {
	switch mt {
	case MTError:
		return "Error"
	case MTWarning:
		return "Warning"
	case Info:
		return "Info"
	case Log:
		return "Log"
	}
	panic("unreached")
}
