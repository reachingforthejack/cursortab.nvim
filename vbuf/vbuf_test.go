package vbuf_test

import (
	"cursortab.nvim/vbuf"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVbufCreation(t *testing.T) {
	s := "line1\nline2\nline3"
	b := vbuf.New(&s, 0, 0)
	assert.Equal(t, 3, b.GetLineCount())

	s2 := "single line"
	b2 := vbuf.New(&s2, 0, 0)
	assert.Equal(t, 1, b2.GetLineCount())

	s3 := ""
	b3 := vbuf.New(&s3, 0, 0)
	assert.Equal(t, 1, b3.GetLineCount())
}

func TestVbufGetLine(t *testing.T) {
	s := "line1\nline2\nline3"
	b := vbuf.New(&s, 0, 0)
	assert.Equal(t, "line1\n", b.GetLine(0))
	assert.Equal(t, "line2\n", b.GetLine(1))
	assert.Equal(t, "line3", b.GetLine(2))
	assert.Equal(t, "", b.GetLine(3))
}

func TestVbufDiffOutsideRange(t *testing.T) {
	s := "one\ntwo\nthree\nfour"
	b := vbuf.New(&s, 0, 0)
	diffS := "xyz\nabc"
	d := vbuf.New(&diffS, 0, 0)
	vd := vbuf.NewVbufDiff(b, d, 1)
	r1 := vd.DiffResultAtLine(0)
	assert.Equal(t, vbuf.DiffTypeContinuation, r1.Type)
	assert.Equal(t, "", r1.Diff)
	r2 := vd.DiffResultAtLine(3)
	assert.Equal(t, vbuf.DiffTypeContinuation, r2.Type)
	assert.Equal(t, "", r2.Diff)
}

func TestVbufDiffWithinRange(t *testing.T) {
	s := "one\ntwo\nthree\nfour"
	b := vbuf.New(&s, 0, 0)
	diffS := "two-edited\nthree-edited\nxxx"
	d := vbuf.New(&diffS, 0, 0)
	vd := vbuf.NewVbufDiff(b, d, 1)
	r1 := vd.DiffResultAtLine(1)
	assert.NotEqual(t, vbuf.DiffTypeContinuation, r1.Type)
	r2 := vd.DiffResultAtLine(2)
	assert.NotEqual(t, vbuf.DiffTypeContinuation, r2.Type)
	r3 := vd.DiffResultAtLine(3)
	assert.NotEqual(t, vbuf.DiffTypeContinuation, r3.Type)
}

func TestCompareStrings(t *testing.T) {
	tests := []struct {
		name  string
		s1    string
		s2    string
		dType vbuf.DiffType
		diff  string
	}{
		{"SameStrings", "abc", "abc", vbuf.DiffTypeContinuation, ""},
		{"EmptyVsNonEmpty", "", "abc", vbuf.DiffTypeContinuation, "abc"},
		{"Prefix", "abc", "abcd", vbuf.DiffTypeContinuation, "d"},
		{"Replacement", "abc", "xyz", vbuf.DiffTypeReplacement, "xyz"},
		{"InlineContinuation", "abc", "abxyzc", vbuf.DiffTypeInlineContinuation, "xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := vbuf.CompareStrings(tt.s1, tt.s2)
			assert.Equal(t, tt.dType, res.Type)
			assert.Equal(t, tt.diff, res.Diff)
		})
	}
}
