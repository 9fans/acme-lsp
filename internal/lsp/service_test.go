package lsp

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestTextDocumentSyncOptionsOrKind_MarshalUnmarshalJSON(t *testing.T) {
	kindPtr := func(kind TextDocumentSyncKind) *TextDocumentSyncKind {
		return &kind
	}

	tests := []struct {
		data []byte
		want *TextDocumentSyncOptionsOrKind
	}{
		{
			data: []byte(`2`),
			want: &TextDocumentSyncOptionsOrKind{
				Options: &TextDocumentSyncOptions{
					OpenClose: true,
					Change:    TDSKIncremental,
				},
				Kind: kindPtr(2),
			},
		},
		{
			data: []byte(`{"openClose":true,"change":1,"save":{"includeText":true}}`),
			want: &TextDocumentSyncOptionsOrKind{
				Options: &TextDocumentSyncOptions{
					OpenClose: true,
					Change:    TDSKFull,
					Save:      &SaveOptions{IncludeText: true},
				},
			},
		},
	}
	for _, test := range tests {
		var got TextDocumentSyncOptionsOrKind
		if err := json.Unmarshal(test.data, &got); err != nil {
			t.Error(err)
			continue
		}
		if !reflect.DeepEqual(&got, test.want) {
			t.Errorf("got %+v, want %+v", got, test.want)
			continue
		}
		data, err := json.Marshal(&got)
		if err != nil {
			t.Error(err)
			continue
		}
		if !bytes.Equal(data, test.data) {
			t.Errorf("got JSON %q, want %q", data, test.data)
		}
	}
}

func TestMarkupContent_MarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		data        []byte
		want        MarkupContent
		skipMarshal bool
	}{{
		data:        []byte(`{"language":"go","value":"foo"}`),
		want:        MarkupContent{Kind: PlainText, Value: "foo"},
		skipMarshal: true,
	}, {
		data:        []byte(`{"language":"","value":"foo"}`),
		want:        MarkupContent{Kind: PlainText, Value: "foo"},
		skipMarshal: true,
	}, {
		data:        []byte(`"foo"`),
		want:        MarkupContent{Kind: PlainText, Value: "foo"},
		skipMarshal: true,
	}, {
		data:        []byte(`["foo", "bar"]`),
		want:        MarkupContent{Kind: PlainText, Value: "foo\nbar"},
		skipMarshal: true,
	}, {
		data:        []byte(`[{"language":"go","value":"foo"},{"language":"go","value":"bar"}]`),
		want:        MarkupContent{Kind: PlainText, Value: "foo\nbar"},
		skipMarshal: true,
	}, {
		data:        []byte(`{"kind":"markdown","value":"foo"}`),
		want:        MarkupContent{Kind: Markdown, Value: "foo"},
		skipMarshal: false,
	}}

	for _, test := range tests {
		var m MarkupContent
		if err := json.Unmarshal(test.data, &m); err != nil {
			t.Errorf("json.Unmarshal error: %s", err)
			continue
		}
		if !reflect.DeepEqual(test.want, m) {
			t.Errorf("Unmarshaled %q, expected %+v, but got %+v", string(test.data), test.want, m)
			continue
		}

		if !test.skipMarshal {
			marshaled, err := json.Marshal(m)
			if err != nil {
				t.Errorf("json.Marshal error: %s", err)
				continue
			}
			if string(marshaled) != string(test.data) {
				t.Errorf("Marshaled result expected %s, but got %s", string(test.data), string(marshaled))
			}
		}
	}
}

func TestHover(t *testing.T) {
	tests := []struct {
		data          []byte
		want          Hover
		skipUnmarshal bool
		skipMarshal   bool
	}{{
		data:        []byte(`{"contents":[{"language":"go","value":"foo"}]}`),
		want:        Hover{Contents: MarkupContent{Kind: PlainText, Value: "foo"}},
		skipMarshal: true,
	}, {
		data: []byte(`{"contents":{"kind":"markdown","value":"foo"},"range":{"start":{"line":42,"character":5},"end":{"line":42,"character":12}}}`),
		want: Hover{
			Contents: MarkupContent{
				Kind:  Markdown,
				Value: "foo",
			},
			Range: &Range{
				Start: Position{
					Line:      42,
					Character: 5,
				},
				End: Position{
					Line:      42,
					Character: 12,
				},
			},
		},
	}}

	for _, test := range tests {
		if !test.skipUnmarshal {
			var h Hover
			if err := json.Unmarshal(test.data, &h); err != nil {
				t.Errorf("json.Unmarshal %q error: %s", test.data, err)
				continue
			}
			if !reflect.DeepEqual(test.want.Contents, h.Contents) {
				t.Errorf("Unmarshaled %q, expected %#v, but got %#v", string(test.data), test.want.Contents, h.Contents)
				continue
			}
			if !reflect.DeepEqual(test.want.Range, h.Range) {
				t.Errorf("Unmarshaled %q, expected %#v, but got %#v", string(test.data), test.want.Range, h.Range)
				continue
			}
		}

		if !test.skipMarshal {
			marshaled, err := json.Marshal(&test.want)
			if err != nil {
				t.Errorf("json.Marshal error: %s", err)
				continue
			}
			if string(marshaled) != string(test.data) {
				t.Errorf("Marshaled result expected %s, but got %s", string(test.data), string(marshaled))
			}
		}
	}
}

func TestFormattingOptions(t *testing.T) {
	tests := []struct {
		data []byte
		opt  FormattingOptions
	}{
		{
			data: []byte(`{"tabSize":0,"insertSpaces":false,"key":{}}`),
			opt: FormattingOptions{
				TabSize:      0,
				InsertSpaces: false,
				Key:          map[string]interface{}{},
			},
		},
	}
	for _, test := range tests {
		var opt FormattingOptions
		if err := json.Unmarshal(test.data, &opt); err != nil {
			t.Errorf("json.Unmarshal %q error: %s", test.data, err)
			continue
		}
		if !reflect.DeepEqual(test.opt, opt) {
			t.Errorf("Unmarshaled %q, expected %#v, but got %#v", string(test.data), test.opt, opt)
			continue
		}

		marshaled, err := json.Marshal(&test.opt)
		if err != nil {
			t.Errorf("json.Marshal error: %s", err)
			continue
		}
		if string(marshaled) != string(test.data) {
			t.Errorf("Marshaled result expected %s, but got %s", string(test.data), string(marshaled))
		}
	}
}

func TestMessageTypeString(t *testing.T) {
	for _, test := range []struct {
		m MessageType
		s string
	}{
		{MTError, "Error"},
		{MTWarning, "Warning"},
		{Info, "Info"},
		{Log, "Log"},
	} {
		s := test.m.String()
		if s != test.s {
			t.Errorf("String representation of MessageType(%v) is %v; expected %v",
				int(test.m), s, test.s)
		}
	}
}
