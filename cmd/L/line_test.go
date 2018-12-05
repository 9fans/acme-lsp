package main

import (
	"bytes"
	"testing"
)

const testFile1 = `123
56αβ9

CDE
`

// testFile2 describes a file that does not end with '\n'
const testFile2 = `12345
678`

func TestLineOffsets(t *testing.T) {
	var testCases = []struct {
		file              string
		offset, line, col int
	}{
		{testFile1, 0x0, 0, 0},
		{testFile1, 0x1, 0, 1},
		{testFile1, 0x2, 0, 2},
		{testFile1, 0x4, 1, 0},
		{testFile1, 0x5, 1, 1},
		{testFile1, 0x6, 1, 2},
		{testFile1, 0x7, 1, 3},
		{testFile1, 0x8, 1, 4},
		{testFile1, 0x9, 1, 5},
		{testFile1, 0xA, 2, 0},
		{testFile1, 0xB, 3, 0},
		{testFile1, 0xC, 3, 1},
		{testFile1, 0xD, 3, 2},
		{testFile1, 0xE, 3, 3},
		{testFile1, 0xF, 4, 0},
		{testFile2, 0x5, 0, 5},
		{testFile2, 0x6, 1, 0},
		{testFile2, 0x7, 1, 1},
		{testFile2, 0x8, 1, 2},
	}

	for _, tc := range testCases {
		off, err := getNewlineOffsets(bytes.NewBufferString(tc.file))
		if err != nil {
			t.Errorf("failed to compute file offsets: %v", err)
			continue
		}
		if o := off.LineToOffset(tc.line, tc.col); o != tc.offset {
			t.Errorf("LineToOffset(%v, %v) = %v; expected %v\n",
				tc.line, tc.col, o, tc.offset)
		}
		if line, col := off.OffsetToLine(tc.offset); line != tc.line || col != tc.col {
			t.Errorf("OffsetToLine(%v) = %v, %v; expected %v, %v\n",
				tc.offset, line, col, tc.line, tc.col)
		}
	}
}
