package rpcplugin

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"cursortab.nvim/cursortab"
	"github.com/neovim/go-client/nvim"
)

type textOpQueue struct {
	sync.Mutex
	op textOp
}

func newTextOpQueue() *textOpQueue {
	return &textOpQueue{}
}

func (q *textOpQueue) peekOp() textOp {
	q.Lock()
	defer q.Unlock()

	return q.op
}

func (q *textOpQueue) pushOp(v *nvim.Nvim, op textOp) {
	if op == nil {
		return
	}

	q.Lock()
	defer q.Unlock()

	if q.op != nil {
		q.op.clearPreview(v)
	}
	q.op = nil

	op.applyPreview(v)
	q.op = op
}

func (q *textOpQueue) popOp(v *nvim.Nvim) {
	q.Lock()
	defer q.Unlock()

	if q.op == nil {
		return
	}

	op := q.op
	q.op = nil

	op.clearPreview(v)

	if applied := op.applyAction(v); !applied {
		repKey, err := v.ReplaceTermcodes("<Tab>", true, false, true)
		if err != nil {
			log.Printf("failed to replace termcodes: %v\n", err)
			return
		}
		v.FeedKeys(repKey, "n", true)
	}
}

type textOp interface {
	applyPreview(v *nvim.Nvim)
	applyAction(v *nvim.Nvim) bool
	clearPreview(v *nvim.Nvim)
}

type cursorJumpOp struct {
	*cursortab.TextJump
}

func (o *cursorJumpOp) applyPreview(v *nvim.Nvim) {
	if o.TextJump == nil || o.OutOfRange {
		return
	}
	if err := v.ExecLua(fmt.Sprintf("PlaceCursortabSign(%v)", o.LineNumber+1), nil, nil); err != nil {
		log.Printf("failed to execute lua command: %v\n", err)
	}
}

func (o *cursorJumpOp) applyAction(v *nvim.Nvim) bool {
	if o.TextJump == nil || o.OutOfRange {
		return false
	}
	if err := v.Command(fmt.Sprintf(":%v", o.LineNumber+1)); err != nil {
		log.Printf("failed to set window cursor: %v\n", err)
		return false
	}
	return true
}

func (o *cursorJumpOp) clearPreview(v *nvim.Nvim) {
	if err := v.ExecLua("ClearCursortabSign()", nil, nil); err != nil {
		log.Printf("failed to execute lua command: %v\n", err)
	}
}

// textPatchOp is an operation that replaces text in the buffer.
type textPatchOp struct {
	*cursortab.TextPatch
	bufLines []string
}

func (o *textPatchOp) applyPreview(v *nvim.Nvim) {
	if o.TextPatch == nil || o.Range == nil {
		return
	}
	contentLines := strings.Split(o.Content, "\n")
	for i := o.Range.StartLine; i < o.Range.EndLineInc-1; i++ {
		line := contentLines[i-o.Range.StartLine]
		originLine := ""
		if len(o.bufLines) > i-1 {
			originLine = o.bufLines[i-1]
		}

		if strings.Contains(line, originLine) {
			log.Printf("origin line %v: %v\n", i, originLine)

			line = strDifference(originLine, line)
			line = escapeLuaString(line)

			if err := v.ExecLua(
				fmt.Sprintf("PreviewLineContent(%v, %v, \"%v\")", i, len(originLine), line),
				nil,
				nil,
			); err != nil {
				log.Printf("failed to execute lua command: %v\n", err)
				return
			}
		} else {
			line = strDifference(originLine, line)
			line = escapeLuaString(line)

			if err := v.ExecLua(
				fmt.Sprintf("PreviewReplaceContent(%v, \"%v\")", i, line),
				nil,
				nil,
			); err != nil {
				log.Printf("failed to execute lua command: %v\n", err)
				return
			}
		}
	}
}

func (o *textPatchOp) applyAction(v *nvim.Nvim) bool {
	if o.TextPatch == nil || o.Range == nil {
		log.Printf("invalid text patch operation: %v\n", o)
		return false
	}
	log.Printf("applying text patch operation: %+v\n", *o)

	contentLines := strings.Split(o.Content, "\n")
	jumpLine := 0

	for i := o.Range.StartLine; i < o.Range.EndLineInc-1; i++ {
		line := contentLines[i-o.Range.StartLine]
		originLine := o.bufLines[i-1]

		lineDiff := strDifference(originLine, line)

		log.Printf("line %v: %v", i, line)
		log.Printf("origin line %v: %v\n", i, originLine)
		log.Printf("line diff %v: %v\n========", i, lineDiff)

		if lineDiff == "" {
			log.Printf("no diff, skipping")
			continue

		}

		jumpLine = i
		log.Printf("jump line %v\n", jumpLine)

		if err := v.Call("setline", nil, i, line); err != nil {
			log.Printf("failed to set line %v: %v\n", i, err)
			return false
		}
	}

	if err := v.Command(fmt.Sprintf(":%v", jumpLine)); err != nil {
		log.Printf("failed to set cursor to ending line: %v\n", err)
	}

	if err := v.SetWindowCursor(0, [2]int{jumpLine, 999999}); err != nil {
		log.Printf("failed to set cursor to end of line: %v\n", err)
	}

	return true
}

func (o *textPatchOp) peekMaybeExtensionAtEndOfLine(v *nvim.Nvim) (bool, []string) {
	if o.TextPatch == nil || o.Range == nil {
		return false, nil
	}

	contentLines := strings.Split(o.Content, "\n")
	expected := o.Range.EndLineInc - o.Range.StartLine - 1
	if len(contentLines) != expected {
		return false, nil
	}

	appendedDiffs := make([]string, expected)

	for i := 0; i < expected; i++ {
		oldLineIdx := o.Range.StartLine + i
		var oldLine string
		if err := v.Call("getline", &oldLine, oldLineIdx); err != nil {
			return false, nil
		}
		newLine := contentLines[i]

		if len(newLine) <= len(oldLine) {
			return false, nil
		}
		if !strings.HasPrefix(newLine, oldLine) {
			return false, nil
		}

		appendedDiff := newLine[len(oldLine):]
		appendedDiffs[i] = appendedDiff
	}

	return true, appendedDiffs
}

func (o *textPatchOp) clearPreview(v *nvim.Nvim) {
	if o == nil || o.Range == nil {
		return
	}
	for i := o.Range.StartLine; i < o.Range.EndLineInc-1; i++ {
		if err := v.ExecLua(fmt.Sprintf("ClearPreviewLineContent(%v)", i), nil, nil); err != nil {
			log.Printf("failed to execute lua command: %v\n", err)
		}
	}
}

func escapeLuaString(str string) string {
	return strings.ReplaceAll(str, `"`, `\"`)
}

// given two individual lines, return the difference at the tail.
// for example with base = "Hello, " and other = "Hello, world!"
// return " world!"
func strDifference(base, other string) string {
	commonPrefixLen := 0
	minLen := len(base)
	if len(other) < minLen {
		minLen = len(other)
	}

	for i := 0; i < minLen; i++ {
		if base[i] != other[i] {
			break
		}
		commonPrefixLen++
	}

	return other[commonPrefixLen:]
}
