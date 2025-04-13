package main

import (
	"fmt"
	"log"

	"github.com/neovim/go-client/nvim"
)

type buffer struct {
	lines       []string
	row         int
	col         int
	path        string
	version     int
	id          nvim.Buffer
	diffHistory []string
}

func newBuffer() (*buffer, error) {
	log.Printf("creating new buffer")

	return &buffer{
		lines:       []string{},
		row:         0,
		col:         0,
		path:        "",
		version:     0,
		id:          nvim.Buffer(0),
		diffHistory: []string{},
	}, nil
}

func (b *buffer) syncIn(v *nvim.Nvim) {
	currentBuf, err := v.CurrentBuffer()
	if err != nil {
		log.Printf("error getting current buffer: %v", err)
		return
	}

	path, err := v.BufferName(currentBuf)
	if err != nil {
		log.Printf("error getting buffer name: %v", err)
		return
	}

	lines, err := v.BufferLines(currentBuf, 0, -1, false)
	if err != nil {
		log.Printf("error getting buffer lines: %v", err)
		return
	}

	linesStr := make([]string, len(lines))
	for i, line := range lines {
		linesStr[i] = string(line[:])
	}

	window, err := v.CurrentWindow()
	if err != nil {
		log.Printf("error getting current window: %v", err)
		return
	}

	cursor, err := v.WindowCursor(window)
	if err != nil {
		log.Printf("error getting window cursor: %v", err)
		return
	}

	b.lines = linesStr
	b.row = cursor[1]
	b.col = cursor[0] - 1

	log.Printf("synced col: %v, row: %v", b.col, b.row)

	b.path = path
	if b.id != currentBuf {
		b.id = currentBuf
		b.diffHistory = []string{}
		b.version = 0
	}
}

func (b *buffer) editLines(v *nvim.Nvim, applyBatch *nvim.Batch, nsID, startLine, endLineInclusive int, place []string) *nvim.Batch {
	batch := v.NewBatch()

	b.clearNamespace(batch, nsID)

	log.Printf("editing lines %d..%d in buffer %d", startLine, endLineInclusive, b.id)

	log.Printf("applied line modification: %v", nsID)

	dummyIdRxPtr := 0
	diffStr := ""
	lastModifiedLine := 0

	for i := startLine; i <= endLineInclusive; i++ {
		relativeLineIdx := i - startLine

		var l *string
		var realL *string

		if i < len(b.lines) {
			realL = &b.lines[i]
		}

		log.Printf("relativeLineIdx: %d", relativeLineIdx)

		if relativeLineIdx < len(place) {
			log.Printf("placing line %d", relativeLineIdx)
			l = &place[relativeLineIdx]
		}

		if l != nil && realL != nil && *l == *realL {
			log.Printf("lines identical so skipping preview")
			continue
		}

		lastModifiedLine = i

		if l != nil && realL != nil && *l != *realL {
			log.Printf("lines different so adding preview")
			diffStr += fmt.Sprintf("%d-|%s\n", i+1, *realL)
			diffStr += fmt.Sprintf("%d+|%s\n", i+1, *l)
		} else if l != nil && realL == nil {
			log.Printf("adding new line to preview")
			diffStr += fmt.Sprintf("%d+|%s\n", i+1, *l)
		} else if l == nil && realL != nil {
			log.Printf("removing line from preview")
			diffStr += fmt.Sprintf("%d-|%s\n", i+1, *realL)
		}

		if l != nil && realL != nil {
			log.Printf("adding buffer extmark + hl to line %d", i)
			batch.SetBufferExtmark(b.id, nsID, i, 0, map[string]any{
				"virt_text":     []any{[]any{*l, "cursortabhl_addition"}},
				"virt_text_pos": "eol",
				"hl_mode":       "combine",
			}, &dummyIdRxPtr)

			batch.AddBufferHighlight(b.id, nsID, "cursortabhl", i, 0, -1, &dummyIdRxPtr)

			log.Printf("added buffer extmark + hl to line %d", i)
		} else {
			log.Printf("nothing to preview")
		}
	}

	if len(place) >= startLine-lastModifiedLine {
		lastModifiedLine = startLine + len(place) - 1
	}

	log.Printf("diffStr: %s", diffStr)

	b.diffHistory = append(b.diffHistory, diffStr)
	if len(b.diffHistory) > 3 {
		b.diffHistory = b.diffHistory[len(b.diffHistory)-3:]
	}

	if err := batch.Execute(); err != nil {
		log.Printf("error executing hl batch: %v", err)
		return nil
	}

	if applyBatch == nil {
		applyBatch = v.NewBatch()
	}

	b.clearNamespace(applyBatch, nsID)

	log.Printf("applying to buffer %d (%d..%d)", b.id, startLine, endLineInclusive)

	placeBytes := make([][]byte, len(place))
	for i, line := range place {
		placeBytes[i] = []byte(line)
	}

	// execute lua to actually clear the lines within the range beforehand
	applyBatch.ExecLua(fmt.Sprintf("vim.cmd('normal! %v,%vd')", startLine+1, endLineInclusive+1), nil, nil)

	applyBatch.SetBufferLines(b.id, startLine, endLineInclusive, false, placeBytes)

	if lastModifiedLine > 0 {
		applyBatch.SetWindowCursor(0, [2]int{lastModifiedLine + 1, 0})
		applyBatch.ExecLua("vim.cmd('normal! zz')", nil, nil)
		applyBatch.ExecLua("vim.cmd('normal! 1000l')", nil, nil)
	}

	b.version++

	return applyBatch
}

func (b *buffer) setCursorPosition(v *nvim.Nvim, nsID, col int) *nvim.Batch {
	applyBatch := v.NewBatch()
	applyBatch.SetWindowCursor(0, [2]int{col, 0})

	dummyIdRxPtr := 0

	applyBatch.AddBufferHighlight(b.id, nsID, "cursortabhl_yellowish", col, 0, -1, &dummyIdRxPtr)

	return applyBatch
}

func (b *buffer) clearNamespace(batch *nvim.Batch, nsID int) {
	batch.ClearBufferNamespace(b.id, nsID, 0, -1)
}
