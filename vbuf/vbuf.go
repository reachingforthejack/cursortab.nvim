package vbuf

import (
	"fmt"
	"log"
	"strings"
)

// Vbuf represents a virtual buffer of the text buffer currently
// being edited. This exists for two reasons:
// 1. Memory and performance so we aren't copying around a potentially huge buffer,
// instead we let the caller pick one "source" which is a pointer to a string stored here
// 2. To reconcile indexing differences. Both Cursor's API and Nvim use strange non-zero-based
// indexing that we reconcile in here.
type Vbuf struct {
	// pointer to the source buffer
	source *string
	// index of each line's starting position
	lineIndexes []int
	// zero-indexed line position
	userLinePos int
	// zero-indexed col position
	userColPos int
}

// New creates a new Vbuf from a source string.
// IMPORTANT: this source string should _not_ change
// from under this, or else the line calculation will be wrong.
func New(source *string, userLinePos, userColPos int) Vbuf {
	lineIndexes := []int{0}

	for i, c := range *source {
		if c == '\n' {
			lineIndexes = append(lineIndexes, i+1)
		}
	}

	return Vbuf{
		source:      source,
		lineIndexes: lineIndexes,
		userLinePos: userLinePos,
		userColPos:  userColPos,
	}
}

// GetLineCount returns the number of lines in the buffer.
func (b Vbuf) GetLineCount() int {
	return len(b.lineIndexes)
}

// GetUserLinePos returns the zero-indexed line position of the user
func (b Vbuf) GetUserLinePos() int {
	return b.userLinePos
}

// GetLine returns the specified line and strips the trailing newline char
func (b Vbuf) GetLine(line int) string {
	if line < 0 || line >= len(b.lineIndexes) {
		return ""
	}

	start := b.lineIndexes[line]
	var end int
	if line == len(b.lineIndexes)-1 {
		end = len(*b.source)
	} else {
		end = b.lineIndexes[line+1]
	}

	if end < start {
		end = start
	}

	return strings.TrimRight((*b.source)[start:end], "\n")
}

func (b Vbuf) GetCurrentLine() string {
	return b.GetLine(b.userLinePos - 1)
}

// debug the line count and then positions
func (v Vbuf) DebugString() string {
	return fmt.Sprintf("Vbuf: %d lines, userLinePos: %d, userColPos: %d", len(v.lineIndexes), v.userLinePos, v.userColPos)
}

// VbufDiff represents a diff on a slice of the Vbuf. This is a composition
// of two vbufs, the base and the diff, along with a line offset and
// a count of how many lines in the base are replaced.
type VbufDiff struct {
	// vbuf is the base, full vbuf
	vbuf *Vbuf
	// diffVbuf is a vbuf that _only_ represents the changed lines,
	// meaning this is not a full buffer
	diffVbuf Vbuf
	// the offset from the base vbuf for which the diff begins
	startingLineOffset int
	endLineInclusive   int
	replacedLineCount  int
}

// NewVbufDiff creates a new VbufDiff from a base vbuf, a diff vbuf,
// a line offset, and a replaced line count.
func NewVbufDiff(base *Vbuf, diff Vbuf, startLineOffset, endLineOffset int) VbufDiff {
	replacedLineCount := diff.GetLineCount()
	return VbufDiff{
		vbuf:               base,
		diffVbuf:           diff,
		startingLineOffset: startLineOffset,
		endLineInclusive:   endLineOffset,
		replacedLineCount:  replacedLineCount,
	}
}

func (d VbufDiff) DiffLine(line int) string {
	return d.diffVbuf.GetLine(line)
}

func (d VbufDiff) DiffLines() []string {
	lc := d.diffVbuf.GetLineCount()
	lines := make([]string, 0)
	for i := 0; i < lc; i++ {
		lines = append(lines, d.diffVbuf.GetLine(i))
	}
	return lines
}

// DiffLines may return lines that didn't actually change. By checking diff lines against
// known lines, we can figure out the last line that was actually different.
func (d VbufDiff) IndexOfFinalChangedLine(precomputedDiffLines []string) int {
	lastChangedLine := 0
	for i := 0; i < len(precomputedDiffLines); i++ {
		if len(d.vbuf.GetLine(d.startingLineOffset+i-1)) < i {
			log.Printf("line %d is empty", d.startingLineOffset+i)
			lastChangedLine = i
			continue
		}

		log.Printf("comparing %s with %s", precomputedDiffLines[i], d.vbuf.GetLine(d.startingLineOffset+i-1))

		if precomputedDiffLines[i] != d.vbuf.GetLine(d.startingLineOffset+i-1) {
			lastChangedLine = i
		}
	}
	return d.startingLineOffset + lastChangedLine
}

func (d VbufDiff) DiffResultAtLine(line int) *DiffResult {
	if line < d.startingLineOffset || line >= d.startingLineOffset+d.replacedLineCount {
		return &DiffResult{
			Type: DiffTypeContinuation,
			Diff: "",
		}
	}

	diffLine := line - d.startingLineOffset
	diffText := d.diffVbuf.GetLine(diffLine)
	baseText := d.vbuf.GetLine(line - 1)

	dr := CompareStrings(baseText, diffText)

	// if there's a continuation with no diff, we don't want to do anything with this
	if dr.Type == DiffTypeContinuation && dr.Diff == "" {
		return nil
	}

	return &dr
}

func (d VbufDiff) StartingLineOffset() int {
	return d.startingLineOffset
}

func (d VbufDiff) EndLineInclusive() int {
	return d.endLineInclusive
}

func (d VbufDiff) ReplacedLineCount() int {
	return d.replacedLineCount
}

func (d VbufDiff) DebugString() string {
	return fmt.Sprintf("VbufDiff:\n```\n%v```\n\n %d lines, startingLineOffset: %d, endingLineOffset: %d, replacedLineCount: %d", *d.diffVbuf.source, d.vbuf.GetLineCount(), d.startingLineOffset, d.endLineInclusive, d.replacedLineCount)
}

func (d VbufDiff) UserColPos() int {
	return d.vbuf.userColPos
}

type DiffResult struct {
	Type DiffType
	Diff string
}

// CompareStrings compares two strings and produces a diff result representing
// one of three possible diffs
func CompareStrings(s1, s2 string) DiffResult {
	if s1 == s2 {
		return DiffResult{
			Type: DiffTypeContinuation,
			Diff: "",
		}
	}
	if len(s1) == 0 {
		return DiffResult{
			Type: DiffTypeContinuation,
			Diff: s2,
		}
	}

	// if s1 is strictly a prefix of s2 then it must be a continuation
	if len(s2) > len(s1) && s2[:len(s1)] == s1 {
		return DiffResult{
			Type: DiffTypeContinuation,
			Diff: s2[len(s1):],
		}
	}

	// otherwise, we have to determine between an inline continuation or a replacement.
	// by finding the longest common prefix and suffix, we can compare the middle part
	// and then see if those differ by insertion in the middle or if its an entire replacement
	prefixLen := longestCommonPrefix(s1, s2)
	suffixLen := longestCommonSuffix(s1, s2, prefixLen)

	midS1 := s1[prefixLen : len(s1)-suffixLen]
	midS2 := s2[prefixLen : len(s2)-suffixLen]

	if len(midS1) == 0 && len(midS2) > 0 {
		return DiffResult{
			Type: DiffTypeInlineContinuation,
			Diff: midS2,
		}
	}

	return DiffResult{
		Type: DiffTypeReplacement,
		Diff: s2,
	}
}

func longestCommonPrefix(s1, s2 string) int {
	minLen := min(len(s1), len(s2))
	i := 0
	for i < minLen && s1[i] == s2[i] {
		i++
	}
	return i
}

func longestCommonSuffix(s1, s2 string, prefixLen int) int {
	remainingS1 := len(s1) - prefixLen
	remainingS2 := len(s2) - prefixLen
	i := 0
	for i < remainingS1 && i < remainingS2 {
		if s1[len(s1)-1-i] != s2[len(s2)-1-i] {
			break
		}
		i++
	}
	return i
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type DiffType int

const (
	DiffTypeContinuation DiffType = iota
	DiffTypeInlineContinuation
	DiffTypeReplacement
)
