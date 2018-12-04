package main

import (
	"bufio"
	"io"
	"unicode/utf8"
)

type nlOffsets []int

func getNewlineOffsets(r io.Reader) (nlOffsets, error) {
	br := bufio.NewReader(r)
	o := 0
	off := []int{0}
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF {
			break
		}
		o += utf8.RuneCountInString(line)
		off = append(off, o)
	}
	return off, nil
}

func (off nlOffsets) LineToOffset(line, col int) int {
	if line >= len(off) {
		panic("bad line number")
	}
	return off[line] + col
}

func (off nlOffsets) OffsetToLine(offset int) (line, col int) {
	for i, o := range off {
		if o > offset {
			return i - 1, offset - off[i-1]
		}
	}
	if i := len(off) - 1; offset >= off[i] {
		return i, offset - off[i]
	}
	panic("unreachable")
}
