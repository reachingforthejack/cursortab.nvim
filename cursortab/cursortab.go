package cursortab

import (
	"context"
	"log"
	"strings"
	"sync"

	"cursortab.nvim/vbuf"
	"github.com/everestmz/cursor-rpc"
	aiserverv1 "github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1"
	"github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1/aiserverv1connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

const debugOutput = true

// CursorTab implements the cursor RPC api for tab completions
type CursorTab struct {
	creds             *cursor.CursorCredentials
	service           aiserverv1connect.AiServiceClient
	workspaceRootPath string

	diffHistory []string

	currentFile      *aiserverv1.CurrentFileInfo
	currentFileMutex *sync.Mutex
}

func New(workspaceRootPath string) (*CursorTab, error) {
	var creds *cursor.CursorCredentials
	service := cursor.NewAiServiceClient()

	currentFile := &aiserverv1.CurrentFileInfo{
		RelativeWorkspacePath: "",
		Contents:              "",
		RelyOnFilesync:        false,
		WorkspaceRootPath:     workspaceRootPath,
		CursorPosition: &aiserverv1.CursorPosition{
			Line:   int32(0),
			Column: int32(0),
		},
	}

	currentFileMutex := &sync.Mutex{}
	diffHistory := []string{}

	ct := &CursorTab{
		creds,
		service,
		workspaceRootPath,
		diffHistory,
		currentFile,
		currentFileMutex,
	}

	go func() {
		ct.currentFileMutex.Lock()
		defer ct.currentFileMutex.Unlock()

		creds, err := cursor.GetDefaultCredentials()
		if err != nil {
			log.Fatalf("failed to retrieve credentials: %v", err)
		}

		ct.creds = creds
	}()

	return ct, nil
}

func (c *CursorTab) CurrentFileVbuf() (*vbuf.Vbuf, error) {
	if c.currentFile == nil {
		return nil, errors.New("no current file set")
	}

	vb := vbuf.New(&c.currentFile.Contents, int(c.currentFile.CursorPosition.Line), int(c.currentFile.CursorPosition.Column))
	return &vb, nil
}

func (c *CursorTab) SetCurrentLineNumber(line int) {
	c.currentFileMutex.Lock()
	defer c.currentFileMutex.Unlock()

	if c.currentFile == nil {
		log.Printf("no current file set")
		return
	}

	c.currentFile.CursorPosition.Line = int32(line)
}

func (c *CursorTab) SetCurrentFile(path, contents string, line, col int) *vbuf.Vbuf {
	c.currentFileMutex.Lock()
	defer c.currentFileMutex.Unlock()

	fileExt := strings.ToLower(strings.TrimPrefix(path, "."))
	language := languageFromExtension(fileExt)

	vb := vbuf.New(&contents, line, col)
	log.Printf("vbuf created for current file: %v\n", vb.DebugString())

	if c.currentFile.RelativeWorkspacePath != path {
		c.diffHistory = []string{}
	}

	c.currentFile.Contents = contents
	c.currentFile.LanguageId = language
	c.currentFile.RelativeWorkspacePath = path
	c.currentFile.TotalNumberOfLines = int32(vb.GetLineCount())
	c.currentFile.CursorPosition = &aiserverv1.CursorPosition{
		Line:   int32(line),
		Column: int32(col),
	}

	return &vb
}

func (c *CursorTab) GetCursorPredictions(ctx context.Context) (*TextJump, error) {
	c.currentFileMutex.Lock()
	defer c.currentFileMutex.Unlock()

	if c.currentFile == nil {
		return nil, errors.New("no current file set")
	}

	req := &aiserverv1.StreamNextCursorPredictionRequest{
		CurrentFile:         c.currentFile,
		ModelName:           proto.String(""),
		DiffHistory:         c.diffHistory,
		DiffHistoryKeys:     []string{},
		ContextItems:        []*aiserverv1.CppContextItem{},
		ParameterHints:      []*aiserverv1.CppParameterHint{},
		LspContexts:         []*aiserverv1.LspSubgraphFullContext{},
		WorkspaceId:         proto.String("mock-ws-id-456"),
		FileSyncUpdates:     []*aiserverv1.FilesyncUpdateWithModelVersion{},
		EnableMoreContext:   proto.Bool(true),
		FileDiffHistories:   []*aiserverv1.CppFileDiffHistory{},
		MergedDiffHistories: []*aiserverv1.CppFileDiffHistory{},
		BlockDiffPatches:    []*aiserverv1.BlockDiffPatch{},
		CppIntentInfo: &aiserverv1.CppIntentInfo{
			Source: IntentManualTrigger,
		},
	}

	stream, err := c.service.StreamNextCursorPrediction(ctx, cursor.NewRequest(c.creds, req))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream")
	}

	var tj *TextJump

	for stream.Receive() {
		resp := stream.Msg()
		tj = &TextJump{
			Text:       resp.Text,
			LineNumber: int(resp.LineNumber),
			OutOfRange: resp.IsNotInRange,
			FileName:   resp.FileName,
		}
		log.Printf("received response: %v\n", tj)
	}

	if err := stream.Err(); err != nil {
		return nil, errors.Wrap(err, "stream error")
	}

	return tj, nil
}

func (c *CursorTab) AddDiffHistory(diff string) {
	if len(c.diffHistory) > 5 {
		c.diffHistory = c.diffHistory[1:]
	}
	c.diffHistory = append(c.diffHistory, diff)
}

func (c *CursorTab) GetCompletions(ctx context.Context, reason string) (*vbuf.VbufDiff, error) {
	c.currentFileMutex.Lock()
	defer c.currentFileMutex.Unlock()

	if c.currentFile == nil {
		return nil, errors.New("no current file set")
	}

	req := &aiserverv1.StreamCppRequest{
		CurrentFile:         c.currentFile,
		DiffHistory:         c.diffHistory,
		ModelName:           proto.String("main"),
		FileDiffHistories:   []*aiserverv1.CppFileDiffHistory{},
		MergedDiffHistories: []*aiserverv1.CppFileDiffHistory{},
		BlockDiffPatches:    []*aiserverv1.BlockDiffPatch{},
		GiveDebugOutput:     proto.Bool(debugOutput),
		LspContexts:         []*aiserverv1.LspSubgraphFullContext{},
		ContextItems:        []*aiserverv1.CppContextItem{},
		ParameterHints:      []*aiserverv1.CppParameterHint{},
		CppIntentInfo: &aiserverv1.CppIntentInfo{
			Source: reason,
		},
		WorkspaceId:           proto.String("mock-workspace-id"),
		FilesyncUpdates:       []*aiserverv1.FilesyncUpdateWithModelVersion{},
		TimeSinceRequestStart: 0.0,
		TimeAtRequestSend:     0.0,
	}

	stream, err := c.service.StreamCpp(ctx, cursor.NewRequest(c.creds, req))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream")
	}

	suggestionStartLine := 0
	startLineOffset := 0
	endLineInclusive := 0
	diffSrc := ""

	for stream.Receive() {
		resp := stream.Msg()

		if resp.SuggestionStartLine != nil {
			suggestionStartLine = int(*resp.SuggestionStartLine)
		}

		if resp.RangeToReplace != nil {
			startLineOffset = int(resp.RangeToReplace.StartLineNumber)
			endLineInclusive = int(resp.RangeToReplace.EndLineNumberInclusive)
		}

		if resp.Text != "" {
			diffSrc += resp.Text
		}

		if resp.DoneStream != nil && *resp.DoneStream {
			break
		}
	}

	startLineOffset += suggestionStartLine

	if err := stream.Err(); err != nil {
		return nil, errors.Wrap(err, "stream error")
	}

	baseVbuf, err := c.CurrentFileVbuf()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current file vbuf")
	}

	diff := vbuf.New(&diffSrc, 0, 0)
	fullDiff := vbuf.NewVbufDiff(baseVbuf, diff, startLineOffset, endLineInclusive)
	log.Printf("vbuf diff from completion: %v\n", fullDiff.DebugString())

	return &fullDiff, nil
}

type TextJump struct {
	FileName   string
	LineNumber int
	Text       string
	OutOfRange bool
}

const (
	IntentLineChange       = "line_change"
	IntentTyping           = "typing"
	IntentUnspecified      = "unspecified"
	IntentLinterErrors     = "linter_errors"
	IntentParameterHints   = "parameter_hints"
	IntentCursorPrediction = "cursor_prediction"
	IntentManualTrigger    = "manual_trigger"
	IntentEditorChange     = "editor_change"
	IntentLspSuggestions   = "lsp_suggestions"
)
