package ndog

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanDelimFunc(t *testing.T) {
	split := func(data []byte, delim []byte) [][]byte {
		out := [][]byte{}
		s := bufio.NewScanner(bytes.NewReader(data))
		s.Split(ScanDelimFunc(delim))
		for s.Scan() {
			out = append(out, s.Bytes())
		}
		return out
	}

	assert.Equal(
		t,
		[][]byte{
			[]byte("foo"),
			[]byte("bar"),
		},
		split([]byte("foo\x00bar"), []byte{0}),
	)
	assert.Equal(
		t,
		[][]byte{
			[]byte("foo"),
			[]byte("bar"),
		},
		split([]byte("foo,bar"), []byte{','}),
	)
	assert.Equal(
		t,
		[][]byte{
			[]byte("foo!"),
			[]byte("bar;"),
		},
		split([]byte("foo!!;bar;"), []byte{'!', ';'}),
	)
}
