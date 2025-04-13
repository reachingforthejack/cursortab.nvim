package rpcplugin

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"cursortab.nvim/cursortab"
	"cursortab.nvim/vbuf"
	"github.com/neovim/go-client/nvim"
	"github.com/pkg/errors"
)

type NeovimPlugin struct {
	tab  *cursortab.CursorTab
	nvim *nvim.Nvim

	rpcReqCtx    context.Context
	rpcReqCancel context.CancelFunc
	rpcMutex     sync.Mutex

	pendingDiff *vbuf.VbufDiff
	pendingJump *cursortab.TextJump
}

func New() (*NeovimPlugin, error) {
	n, err := nvim.New(os.Stdin, os.Stdout, os.Stdout, log.Printf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create nvim instance")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &NeovimPlugin{
		nvim:         n,
		rpcReqCtx:    ctx,
		rpcReqCancel: cancel,
		rpcMutex:     sync.Mutex{},
	}, nil
}

func (p *NeovimPlugin) BeginListening() error {
	if err := p.nvim.RegisterHandler("cursortab_init", p.hcbCursortabInit); err != nil {
		return errors.Wrap(err, "failed to register handler: cursortab_init")
	}
	if err := p.nvim.RegisterHandler("cursortab", p.hcbCursortab); err != nil {
		return errors.Wrap(err, "failed to register handler: cursortab")
	}
	if err := p.nvim.RegisterHandler("cursortab_apply", p.hcpApply); err != nil {
		return errors.Wrap(err, "failed to register handler: cursortab_apply")
	}

	return p.nvim.Serve()
}

func (p *NeovimPlugin) hcbCursortabInit() {
	log.Println("cursortab_init called")

	cwd, err := p.nvim.Exec("pwd", true)
	if err != nil {
		log.Printf("failed to get current working directory: %v\n", err)
		return
	}

	if p.tab, err = cursortab.New(cwd); err != nil {
		log.Printf("failed to create CursorTab: %v\n", err)
		return
	}
}

func (p *NeovimPlugin) hcbCursortab() {
	go func() {
		if ok := p.rpcMutex.TryLock(); !ok {
			log.Printf("lock pre-held")
			return
		}

		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in completion goroutine: %v\n", r)
			}

			p.clearAllSuggestions()
			p.previewCurrentSuggestion()
		}()

		defer p.rpcMutex.Unlock()

		p.resetActiveCompletionContext()

		p.loadCurrentFile()

		log.Printf("getting completions")

		vbufDiff, err := p.tab.GetCompletions(p.rpcReqCtx, cursortab.IntentTyping)
		if err != nil {
			log.Printf("failed to get completions: %v\n", err)
			return
		}

		p.pendingDiff = vbufDiff
	}()
}

func (p *NeovimPlugin) computeFusedCursorJumpAndCompletionFollowingAccept() {
	if ok := p.rpcMutex.TryLock(); !ok {
		log.Printf("lock pre-held")
		return
	}
	defer p.rpcMutex.Unlock()

	p.resetActiveCompletionContext()
	p.clearAllSuggestions()

	p.pendingDiff = nil
	p.pendingJump = nil

	log.Printf("getting cursor predictions")

	jump, err := p.tab.GetCursorPredictions(p.rpcReqCtx)
	if err != nil {
		log.Printf("failed to get cursor predictions: %v\n", err)
		return
	}

	if jump.OutOfRange {
		log.Printf("jump out of range: %v\n", jump)
		return
	}

	log.Printf("GOT JUMP HERE: %v\n", jump)
	p.pendingJump = jump
	p.previewJump()
	log.Print("loading current file")

	p.loadCurrentFile()
	p.tab.SetCurrentLineNumber(p.pendingJump.LineNumber)

	log.Printf("getting fused completions")

	vbufDiff, err := p.tab.GetCompletions(p.rpcReqCtx, cursortab.IntentCursorPrediction)
	if err != nil {
		log.Printf("failed to get completions: %v\n", err)
		return
	}

	log.Printf("GOT VBUFDIFF HERE: %v\n", vbufDiff)

	p.pendingDiff = vbufDiff
	p.previewCurrentSuggestion()
}

// loadCurrentFile reads the current buffer and cursor position out of the neovim rpc,
// provides them to cursor tab, and then stashes the vbuf
func (p *NeovimPlugin) loadCurrentFile() *vbuf.Vbuf {
	if p.tab == nil {
		log.Printf("no CursorTab instance to load current file\n")
		return nil
	}

	currentBuf, err := p.nvim.CurrentBuffer()
	if err != nil {
		log.Printf("failed to get current buffer: %v\n", err)
		return nil
	}

	currentPath, err := p.nvim.BufferName(currentBuf)
	if err != nil {
		log.Printf("failed to get current buffer name: %v\n", err)
		return nil
	}

	currentWindow, err := p.nvim.CurrentWindow()
	if err != nil {
		log.Printf("failed to get current window: %v\n", err)
		return nil
	}

	pos, err := p.nvim.WindowCursor(currentWindow)
	if err != nil {
		log.Printf("failed to get window cursor: %v\n", err)
		return nil
	}

	contents := ""
	contentsBytes, err := p.nvim.BufferLines(currentBuf, 0, -1, false)
	if err != nil {
		log.Printf("failed to get buffer lines: %v\n", err)
		return nil
	}

	for _, line := range contentsBytes {
		contents += string(line[:])
		contents += "\n"
	}

	return p.tab.SetCurrentFile(currentPath, contents, pos[0], pos[1])
}

func (p *NeovimPlugin) previewCurrentSuggestion() {
	if p.pendingDiff == nil {
		return
	}

	for i := p.pendingDiff.StartingLineOffset(); i < p.pendingDiff.EndLineInclusive(); i++ {
		diff := p.pendingDiff.DiffResultAtLine(i)
		if diff == nil {
			continue
		}

		var luaCmd string
		switch diff.Type {
		case vbuf.DiffTypeContinuation:
			luaCmd = fmt.Sprintf("require('cursortab').PreviewLineContent(%d, %d, \"%s\")", i, p.pendingDiff.UserColPos(), escapeLuaString(diff.Diff))
		case vbuf.DiffTypeInlineContinuation:
		case vbuf.DiffTypeReplacement:
			luaCmd = fmt.Sprintf("require('cursortab').PreviewReplaceContent(%d, \"%s\")",
				i,
				escapeLuaString(diff.Diff),
			)

		}

		log.Printf("luaCmd: %s\n", luaCmd)
		if err := p.nvim.ExecLua(luaCmd, nil, nil); err != nil {
			log.Printf("failed to preview replaced line content: %v\n", err)
		}
	}
}

func (p *NeovimPlugin) previewJump() {
	log.Printf("previewing jump\n")

	if p.pendingJump == nil {
		return
	}

	log.Printf("pending jump: %v\n", p.pendingJump)

	luaCmd := fmt.Sprintf("require('cursortab').PlaceCursortabSign(%d)", p.pendingJump.LineNumber+1)
	if err := p.nvim.ExecLua(luaCmd, nil, nil); err != nil {
		log.Printf("failed to preview jump: %v\n", err)
	}

	log.Printf("previewed jump\n")
}

func (p *NeovimPlugin) clearAllSuggestions() {
	if p.pendingDiff == nil {
		log.Printf("no vbuf to clear\n")
		return
	}

	luaCmd := fmt.Sprintf("require('cursortab').ClearPreviewLineContent(%d,%d)", p.pendingDiff.StartingLineOffset(), p.pendingDiff.EndLineInclusive())
	if err := p.nvim.ExecLua(luaCmd, nil, nil); err != nil {
		log.Printf("failed to clear previewed line content: %v\n", err)
	}

	luaCmd = fmt.Sprintf("require('cursortab').ClearCursortabSign()")
	if err := p.nvim.ExecLua(luaCmd, nil, nil); err != nil {
		log.Printf("failed to clear cursortab sign: %v\n", err)
	}
}

func (p *NeovimPlugin) hcpApply() {
	if p.pendingDiff == nil {
		return
	}

	defer func() {
		go func() {
			log.Println("computing fused cursor jump and completion following accept")
			p.computeFusedCursorJumpAndCompletionFollowingAccept()
		}()
	}()

	window, err := p.nvim.CurrentWindow()
	if err != nil {
		log.Printf("failed to get current window: %v\n", err)
		return
	}

	if p.pendingJump != nil {
		p.nvim.SetWindowCursor(window, [2]int{p.pendingJump.LineNumber, 10000})
	}
	buffer, err := p.nvim.CurrentBuffer()
	if err != nil {
		log.Printf("failed to get current buffer: %v\n", err)
		return
	}

	startingOffset := p.pendingDiff.StartingLineOffset()
	endingLineOffset := p.pendingDiff.EndLineInclusive()

	log.Printf("starting offset: %d, ending line offset: %d\n", startingOffset, endingLineOffset)
	log.Printf("pending diff: %v\n", p.pendingDiff)

	b := p.nvim.NewBatch()

	lines := p.pendingDiff.DiffLines()
	linesAsBytes := make([][]byte, len(lines))
	linesStr := ""
	for i, line := range lines {
		linesAsBytes[i] = []byte(line)
		linesStr += string(line) + "\n"
	}

	b.SetBufferLines(buffer, startingOffset-1, endingLineOffset, false, linesAsBytes)
	lastChangedLine := p.pendingDiff.IndexOfFinalChangedLine(lines)

	b.SetWindowCursor(window, [2]int{lastChangedLine, 10000})

	if err := b.Execute(); err != nil {
		log.Printf("failed to execute batch: %v\n", err)
		return
	}

	p.tab.AddDiffHistory(linesStr)
}

func (p *NeovimPlugin) resetActiveCompletionContext() {
	if p.rpcReqCancel != nil {
		p.rpcReqCancel()
	}
	p.rpcReqCtx, p.rpcReqCancel = context.WithCancel(context.Background())
}

func escapeLuaString(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}
