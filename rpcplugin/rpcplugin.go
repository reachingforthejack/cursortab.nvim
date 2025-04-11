package rpcplugin

import (
	"context"
	"log"
	"os"

	"cursortab.nvim/cursortab"
	"github.com/neovim/go-client/nvim"
	"github.com/pkg/errors"
)

type NeovimPlugin struct {
	tab  *cursortab.CursorTab
	nvim *nvim.Nvim

	activeCompletionCtx    context.Context
	activeCompletionCancel context.CancelFunc

	queue *textOpQueue
}

func New() (*NeovimPlugin, error) {
	var tab *cursortab.CursorTab
	nvim, err := nvim.New(os.Stdin, os.Stdout, os.Stdout, log.Printf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create nvim instance")
	}

	queue := newTextOpQueue()
	activeCompletionCtx, activeCompletionCancel := context.WithCancel(context.Background())

	return &NeovimPlugin{
		tab,
		nvim,

		activeCompletionCtx,
		activeCompletionCancel,

		queue,
	}, nil
}

func (p *NeovimPlugin) BeginListening() error {
	if err := p.nvim.RegisterHandler("cursortab_init", p.hcbCursortabInit); err != nil {
		return errors.Wrap(err, "failed to register handler")
	}
	if err := p.nvim.RegisterHandler("cursortab", p.hcbCursortab); err != nil {
		return errors.Wrap(err, "failed to register handler")
	}
	if err := p.nvim.RegisterHandler("cursortab_apply", p.hcpApply); err != nil {
		return errors.Wrap(err, "failed to register handler")
	}

	return p.nvim.Serve()
}

func (p *NeovimPlugin) hcbCursortabInit() {
	cwd, err := p.nvim.Exec("pwd", true)
	if err != nil {
		log.Printf("failed to get current working directory: %v\n", err)
		return
	}

	tab, err := cursortab.New(cwd)
	if err != nil {
		log.Printf("failed to create CursorTab: %v\n", err)
		return
	}

	bufs, err := p.nvim.Buffers()
	if err != nil {
		log.Printf("failed to get buffers: %v\n", err)
		return
	}

	for _, buf := range bufs {
		path, err := p.nvim.BufferName(buf)
		if err != nil {
			log.Printf("failed to get buffer name: %v\n", err)
			return
		}

		bufLines, err := p.nvim.BufferLines(buf, 0, -1, false)
		if err != nil {
			log.Printf("failed to get buffer lines: %v\n", err)
			return
		}

		contents := ""
		for _, line := range bufLines {
			contents += string(line[:])
			contents += "\n"
		}

		tab.AddFile(int(buf), path, contents, len(bufLines))
	}

	p.tab = tab
}

func (p *NeovimPlugin) hcbCursortab() {
	p.resetCtx()

	buf, err := p.nvim.CurrentBuffer()
	if err != nil {
		log.Printf("failed to get current buffer: %v\n", err)
		return
	}

	w, err := p.nvim.CurrentWindow()
	if err != nil {
		log.Printf("failed to get current window: %v\n", err)
		return
	}

	pos, err := p.nvim.WindowCursor(w)
	if err != nil {
		log.Printf("failed to get window cursor: %v\n", err)
		return
	}

	p.tab.SetActiveFile(int(buf), pos[1]-1, pos[0])

	bufLines, err := p.nvim.BufferLines(buf, 0, -1, false)
	if err != nil {
		log.Printf("failed to get buffer lines: %v\n", err)
		return
	}

	contents := ""
	for _, line := range bufLines {
		contents += string(line[:])
		contents += "\n"
	}

	if err := p.tab.SetActiveFileContents(contents, len(bufLines)); err != nil {
		log.Printf("failed to set active file contents: %v\n", err)
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in completion goroutine: %v\n", r)
			}
		}()

		cmps, err := p.tab.GetCompletions(p.activeCompletionCtx)
		if err != nil {
			log.Printf("failed to get completions: %v\n", err)
			return
		}
		p.queueTextPatch(cmps)

	}()
}

func (p *NeovimPlugin) queueTextPatch(cmp *cursortab.TextPatch) {
	bufLines, err := p.tab.GetCurrentLines()
	if err != nil {
		log.Printf("failed to get current lines: %v\n", err)
		return
	}
	p.queue.pushOp(p.nvim, &textPatchOp{
		cmp,
		bufLines,
	})
}

func (p *NeovimPlugin) hcpApply() {
	p.queue.popOp(p.nvim)

	go func() {
		// bad, but for now we only want to queue a text jump if its after we
		// just performed a cursor prediction
		jumps, err := p.tab.GetCursorPredictions(p.activeCompletionCtx)
		if err != nil {
			log.Printf("failed to get cursor predictions: %v\n", err)
			return
		}
		p.queueTextJump(jumps)
	}()
}

func (p *NeovimPlugin) queueTextJump(jump *cursortab.TextJump) {
	op := &cursorJumpOp{
		jump,
	}

	p.queue.pushOp(p.nvim, op)
}

func (p *NeovimPlugin) resetCtx() {
	if p.activeCompletionCancel != nil {
		p.activeCompletionCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.activeCompletionCtx = ctx
	p.activeCompletionCancel = cancel
}
