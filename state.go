package main

import (
	v1 "connectrpc/cursor/gen/v1"
	aiserverv1connect "connectrpc/cursor/gen/v1/aiserverv1connect"
	"context"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/neovim/go-client/nvim"
	"google.golang.org/protobuf/proto"
)

type state struct {
	buffer        *buffer
	v             *nvim.Nvim
	service       aiserverv1connect.AiServiceClient
	workspacePath string
	workspaceID   string
	context       context.Context
	cancel        context.CancelFunc
	accessToken   string
	checksum      string

	applyBatch   *nvim.Batch
	applyBatchMu *sync.Mutex
}

func newState() (*state, error) {
	service := newAiServiceClient()
	log.Printf("service created")

	v, err := nvim.New(
		os.Stdin, os.Stdout, os.Stdout, log.Printf,
	)
	if err != nil {
		return nil, err
	}

	log.Printf("nvim created")

	context, cancel := context.WithCancel(context.Background())
	accessToken, err := getAccessToken()
	if err != nil {
		cancel()
		return nil, err
	}
	checksum := generateChecksum("hi")
	workspacePath := "~"

	workspaceID := "a-b-c-d-e-f-g"

	buffer, err := newBuffer()
	if err != nil {
		cancel()
		return nil, err
	}

	log.Printf("buffer created: %v", buffer)

	var applyBatch *nvim.Batch
	applyBatchMu := &sync.Mutex{}

	return &state{
		buffer,
		v,
		service,
		workspacePath,
		workspaceID,
		context,
		cancel,
		accessToken,
		checksum,
		applyBatch,
		applyBatchMu,
	}, nil
}

func (s *state) init() error {
	if err := s.v.RegisterHandler("cursortab_sync", func(v *nvim.Nvim, nsID int) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("recovered from panic: %v", r)
				}
			}()
			s.crunchCppStream(nsID, nil)
		}()
	}); err != nil {
		log.Printf("error registering handler: %v", err)
		return nil
	}

	if err := s.v.RegisterHandler("cursortab_tab_key", func(_ *nvim.Nvim, nsID int) {
		s.tabKey(nsID)
	}); err != nil {
		log.Printf("error registering handler: %v", err)
		return nil
	}

	return s.v.Serve()
}

func (s *state) crunchCppStream(nsID int, applyBatch *nvim.Batch) {
	log.Printf("starting stream")

	if ok := s.applyBatchMu.TryLock(); !ok {
		log.Printf("applyBatch is already in use")
		return
	}

	s.applyBatch = nil
	s.applyBatchMu.Unlock()

	oldCol := s.buffer.col

	s.buffer.syncIn(s.v)
	s.restartContext()

	cursorPos := &v1.CursorPosition{
		Line:   int32(s.buffer.col + 1),
		Column: int32(s.buffer.row),
	}

	version := int32(s.buffer.version)

	currentFile := &v1.CurrentFileInfo{
		Contents:              strings.Join(s.buffer.lines, "\n"),
		CursorPosition:        cursorPos,
		FileVersion:           &version,
		RelativeWorkspacePath: s.buffer.path,
	}

	source := "typing"
	if s.applyBatch != nil {
		source = "cursor_prediction"
	} else if oldCol != s.buffer.col {
		source = "line_changed"
	}

	req := &v1.StreamCppRequest{
		WorkspaceId: &s.workspaceID,
		CurrentFile: currentFile,
		CppIntentInfo: &v1.CppIntentInfo{
			// "line_changed" || "typing"
			Source: source,
		},
		FileDiffHistories: []*v1.CppFileDiffHistory{
			{
				FileName:    s.buffer.path,
				DiffHistory: s.buffer.diffHistory,
			},
		},
		IsDebug:         proto.Bool(false),
		GiveDebugOutput: proto.Bool(false),
	}

	stream, err := s.service.StreamCpp(s.context, newRequest(s.accessToken, s.checksum, req))
	if err != nil {
		log.Printf("error starting stream: %v", err)
		return
	}

	startLine := 0
	endLineInc := 0
	newText := ""

	for stream.Receive() {
		msg := stream.Msg()

		if msg.RangeToReplace != nil {
			startLine = int(msg.RangeToReplace.StartLineNumber - 1)
			endLineInc = int(msg.RangeToReplace.EndLineNumberInclusive - 1)
		}

		if msg.SuggestionStartLine != nil {
			log.Printf("suggestion start line: %v", msg.SuggestionStartLine)
		}

		newText += msg.Text

		if msg.DoneStream != nil && *msg.DoneStream {
			break
		}
	}

	log.Printf("stream finished: %s (%v, %v)", newText, startLine, endLineInc)

	if err := stream.Err(); err != nil {
		log.Printf("stream error: %v", err)
		return
	}

	log.Printf("editing lines: %v, %v", startLine, endLineInc)

	s.applyBatch = s.buffer.editLines(s.v, applyBatch, nsID, startLine, endLineInc, strings.Split(newText, "\n"))
}

func (s *state) predictNextCursorPrediction(nsID int) {
	log.Printf("aquiring cursor prediction lock")

	s.applyBatchMu.Lock()

	s.buffer.syncIn(s.v)
	s.restartContext()

	log.Printf("predicting next cursor prediction")

	version := int32(s.buffer.version)

	cursorPos := &v1.CursorPosition{
		Line:   int32(s.buffer.col + 1),
		Column: int32(s.buffer.row),
	}

	currentFile := &v1.CurrentFileInfo{
		Contents:              strings.Join(s.buffer.lines, "\n"),
		CursorPosition:        cursorPos,
		FileVersion:           &version,
		RelativeWorkspacePath: s.buffer.path,
	}

	req := &v1.StreamNextCursorPredictionRequest{
		CurrentFile:     currentFile,
		DiffHistory:     s.buffer.diffHistory,
		WorkspaceId:     &s.workspaceID,
		IsDebug:         proto.Bool(false),
		GiveDebugOutput: proto.Bool(false),
		CppIntentInfo: &v1.CppIntentInfo{
			Source: "line_changed",
		},
	}

	stream, err := s.service.StreamNextCursorPrediction(s.context, newRequest(s.accessToken, s.checksum, req))
	if err != nil {
		log.Printf("error starting stream: %v", err)

		s.applyBatchMu.Unlock()
		return
	}

	lineNumber := 0

	for stream.Receive() {
		msg := stream.Msg()
		log.Printf("predicted line number: %v", msg.LineNumber)
		lineNumber = int(msg.LineNumber)

		if msg.IsNotInRange {
			lineNumber = 0
			break
		}
	}

	if err := stream.Err(); err != nil {
		log.Printf("cursor predict stream error: %v", err)
		s.applyBatchMu.Unlock()
		return
	}

	if lineNumber != 0 {
		s.applyBatch = s.buffer.setCursorPosition(s.v, nsID, lineNumber-1)
	}
	s.applyBatchMu.Unlock()

	s.crunchCppStream(nsID, s.applyBatch)
}

func (s *state) tabKey(nsID int) {
	log.Printf("aquiring tab key lock")

	s.applyBatchMu.Lock()
	applyBatch := s.applyBatch
	s.applyBatch = nil
	s.applyBatchMu.Unlock()

	if applyBatch == nil {
		log.Printf("no apply batch")
	} else {
		if err := applyBatch.Execute(); err != nil {
			log.Printf("error executing tab key batch: %v", err)
		}

		log.Printf("predicting")

		go s.predictNextCursorPrediction(nsID)
	}
}

func (s *state) restartContext() {
	s.cancel()
	s.context, s.cancel = context.WithCancel(context.Background())
}
