package cursortab

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/everestmz/cursor-rpc"
	aiserverv1 "github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1"
	"github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1/aiserverv1connect"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

const debugOutput = true

type CursorTab struct {
	creds             *cursor.CursorCredentials
	service           aiserverv1connect.AiServiceClient
	projectFiles      map[int]*aiserverv1.CurrentFileInfo
	workspaceRootPath string
	activeFileId      int
	activeFileMutex   sync.Mutex
}

func New(workspaceRootPath string) (*CursorTab, error) {
	creds, err := cursor.GetDefaultCredentials()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve credentials")
	}

	service := cursor.NewAiServiceClient()
	projectFiles := make(map[int]*aiserverv1.CurrentFileInfo)
	activeFileId := -1
	activeFileMutex := sync.Mutex{}

	return &CursorTab{
		creds,
		service,
		projectFiles,
		workspaceRootPath,
		activeFileId,
		activeFileMutex,
	}, nil
}

func (c *CursorTab) AddFile(id int, path, contents string, numLines int) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	fileExt := strings.ToLower(strings.TrimPrefix(path, "."))
	language := languageFromExtension(fileExt)

	f := &aiserverv1.CurrentFileInfo{
		RelativeWorkspacePath: path,
		Contents:              contents,
		RelyOnFilesync:        false,

		TotalNumberOfLines: int32(numLines),

		LanguageId: language, // go is go i don't know what other are :(

		WorkspaceRootPath: c.workspaceRootPath,
	}

	c.projectFiles[id] = f

	if c.activeFileId == -1 {
		c.activeFileId = 0
	}
}

func (c *CursorTab) SetActiveFile(id, pos, line int) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	f, ok := c.projectFiles[id]
	if !ok {
		return
	}

	c.activeFileId = id

	f.CursorPosition = &aiserverv1.CursorPosition{
		Line:   int32(line),
		Column: int32(pos),
	}
}

func (c *CursorTab) SetActiveFileContents(contents string, numLines int) error {
	activeFile, err := c.activeFile()
	if err != nil {
		return errors.Wrap(err, "failed to get active file")
	}

	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	activeFile.Contents = contents
	activeFile.TotalNumberOfLines = int32(numLines)
	return nil
}

func (c *CursorTab) activeFile() (*aiserverv1.CurrentFileInfo, error) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	activeFile, ok := c.projectFiles[c.activeFileId]
	if !ok {
		return nil, errors.New("active file not found")
	}

	return activeFile, nil
}

func (c *CursorTab) GetCursorPredictions(ctx context.Context) (*TextJump, error) {
	af, err := c.activeFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active file")
	}

	req := &aiserverv1.StreamNextCursorPredictionRequest{
		CurrentFile: af,
		ModelName:   proto.String(""),

		DiffHistory:     []string{},
		DiffHistoryKeys: []string{},

		ContextItems:        []*aiserverv1.CppContextItem{},
		ParameterHints:      []*aiserverv1.CppParameterHint{},
		LspContexts:         []*aiserverv1.LspSubgraphFullContext{},
		WorkspaceId:         proto.String("mock-ws-id-456"),
		FileSyncUpdates:     []*aiserverv1.FilesyncUpdateWithModelVersion{},
		EnableMoreContext:   proto.Bool(true),
		FileDiffHistories:   []*aiserverv1.CppFileDiffHistory{},
		MergedDiffHistories: []*aiserverv1.CppFileDiffHistory{},
		BlockDiffPatches:    []*aiserverv1.BlockDiffPatch{},

		CppIntentInfo: &aiserverv1.CppIntentInfo{Source: IntentManualTrigger},
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

func (c *CursorTab) GetCurrentLine() (string, error) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	f, ok := c.projectFiles[c.activeFileId]
	if !ok {
		return "", errors.New("file not found")
	}

	return strings.Split(f.Contents, "\n")[f.CursorPosition.Line-1], nil
}

func (c *CursorTab) GetCurrentLineSlicedAfterCurrentRow() (string, error) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	f, ok := c.projectFiles[c.activeFileId]
	if !ok {
		return "", errors.New("file not found")
	}

	currLine := strings.Split(f.Contents, "\n")[f.CursorPosition.Line-1]

	return currLine[f.CursorPosition.Column-1:], nil
}

func (c *CursorTab) GetCurrentLines() ([]string, error) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	f, ok := c.projectFiles[c.activeFileId]
	if !ok {
		return nil, errors.New("file not found")
	}

	lines := strings.Split(f.Contents, "\n")

	return lines, nil
}

func (c *CursorTab) GetFileContents(id int) (string, error) {
	c.activeFileMutex.Lock()
	defer c.activeFileMutex.Unlock()

	f, ok := c.projectFiles[id]
	if !ok {
		return "", errors.New("file not found")
	}

	return f.Contents, nil
}

func (c *CursorTab) GetCompletions(ctx context.Context) (*TextPatch, error) {
	af, err := c.activeFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active file")
	}

	afId := c.activeFileId

	req := &aiserverv1.StreamCppRequest{
		CurrentFile:         af,
		DiffHistory:         []string{},
		ModelName:           proto.String("main"),
		FileDiffHistories:   []*aiserverv1.CppFileDiffHistory{},
		MergedDiffHistories: []*aiserverv1.CppFileDiffHistory{},
		BlockDiffPatches:    []*aiserverv1.BlockDiffPatch{},
		GiveDebugOutput:     proto.Bool(debugOutput),

		LspContexts:    []*aiserverv1.LspSubgraphFullContext{},
		ContextItems:   []*aiserverv1.CppContextItem{},
		ParameterHints: []*aiserverv1.CppParameterHint{},
		CppIntentInfo: &aiserverv1.CppIntentInfo{
			Source: IntentTyping,
		},

		WorkspaceId: proto.String("mock-workspace-id"),

		FilesyncUpdates: []*aiserverv1.FilesyncUpdateWithModelVersion{},

		TimeSinceRequestStart: 0.0,
		TimeAtRequestSend:     0.0,
	}

	stream, err := c.service.StreamCpp(ctx, cursor.NewRequest(c.creds, req))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream")
	}

	var tp *TextPatch

	for stream.Receive() {
		resp := stream.Msg()

		log.Printf("received response: %v\n", resp)

		var newTp *TextPatch
		if tp != nil {
			newTp = tp
		} else {
			newTp = &TextPatch{
				FileId: int(afId),
			}
		}

		if resp.SuggestionStartLine != nil {
			newTp.StartingLine = int(*resp.SuggestionStartLine)
		}

		if resp.RangeToReplace != nil {
			newTp.Range = &TextPatchRange{
				StartLine:  int(resp.RangeToReplace.StartLineNumber),
				EndLineInc: int(resp.RangeToReplace.EndLineNumberInclusive),
			}
			tp = newTp
		}

		// text streams in as appends, but the rest don't so we can concat the string here
		if resp.Text != "" {
			newTp.Content += resp.Text
			tp = newTp
		}

		if resp.DoneStream != nil && *resp.DoneStream {
			break
		}
	}

	if err := stream.Err(); err != nil {
		return nil, errors.Wrap(err, "stream error")
	}

	log.Printf("final response: %v\n", tp)

	return tp, nil
}

type TextJump struct {
	FileName   string
	LineNumber int
	Text       string
	OutOfRange bool
}

type TextPatch struct {
	FileId       int
	Content      string
	Range        *TextPatchRange
	StartingLine int
}

type TextPatchRange struct {
	StartLine  int
	EndLineInc int
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
