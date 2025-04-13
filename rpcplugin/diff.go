package rpcplugin

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type DiffPart struct {
	Value   string
	Added   bool
	Removed bool
}

type InlineModification struct {
	LineNumber int
	Column     int
	Value      string
}

type FullLineModification struct {
	BeforeLineNumber         int
	IndexInMultilineAddition int
	Value                    string
}

type GhostTextDecoration struct {
	StartColumn int
	EndColumn   int
	ClassName   string
	SomeFlag    int
}

func f1rDiffStub(oldStr, newStr string, diffOptions interface{}) []DiffPart {
	return []DiffPart{
		{Value: oldStr, Removed: true},
		{Value: newStr, Added: true},
	}
}

func uDsStub(removedString, addedString string, someOptions interface{}, flag bool) []DiffPart {
	parts := []DiffPart{}
	if removedString != "" {
		parts = append(parts, DiffPart{Value: removedString, Removed: true})
	}
	if addedString != "" {
		parts = append(parts, DiffPart{Value: addedString, Added: true})
	}
	return parts
}

func DiffCharsGuarded(oldStr, newStr string, diffOptions interface{}) []DiffPart {
	if len(oldStr) > 2000 || len(newStr) > 2000 {
		fmt.Println(
			"BAD BAD BAD BAD BAD. THIS SHOULD NOT HAPPEN. PLEASE FIX THE CPP BUG.",
			"diffChars received strings that were too long. Returning trivial diff.",
			len(oldStr),
			len(newStr),
		)
		return []DiffPart{
			{Value: oldStr, Removed: true},
			{Value: newStr, Added: true},
		}
	}
	return f1rDiffStub(oldStr, newStr, diffOptions)
}

type InlineAndFullLineDiff struct {
	InlineModifications   []InlineModification
	FullLineModifications []FullLineModification
}

func computeInlineAndFullLineDiff(diffArray []DiffPart) InlineAndFullLineDiff {
	originalText := ""
	appendedText := ""
	buffer := ""
	multiLineIndex := 0

	inlineMods := make([]InlineModification, 0)
	lineMods := make([]FullLineModification, 0)

	flushBuffer := func(nextValue string) {
		bufferLines := strings.Split(buffer, "\n")
		lineNumber := len(strings.Split(originalText, "\n")) - 1

		if nextValue != "" && !strings.HasPrefix(nextValue, "\n") && len(bufferLines) > 0 {
			lastLine := bufferLines[len(bufferLines)-1]
			bufferLines = bufferLines[:len(bufferLines)-1]

			var col int
			if len(bufferLines) > 0 {
				col = 1
			} else {
				lines := strings.Split(originalText, "\n")
				lastLen := 0
				if len(lines) > 0 {
					lastLen = len(lines[len(lines)-1])
				}
				col = lastLen + 1
			}

			if lastLine != "" {
				inlineMods = append(inlineMods, InlineModification{
					LineNumber: lineNumber,
					Column:     col,
					Value:      lastLine,
				})
			}
			lineNumber--
		} else {
			if len(bufferLines) > 0 {
				firstLine := bufferLines[0]
				bufferLines = bufferLines[1:]

				if firstLine != "" {
					lines := strings.Split(originalText, "\n")
					lastLen := 0
					if len(lines) > 0 {
						lastLen = len(lines[len(lines)-1])
					}
					inlineMods = append(inlineMods, InlineModification{
						LineNumber: lineNumber,
						Column:     lastLen + 1,
						Value:      firstLine,
					})
				}
			}
		}

		for _, lineVal := range bufferLines {
			lineMods = append(lineMods, FullLineModification{
				BeforeLineNumber:         lineNumber,
				IndexInMultilineAddition: multiLineIndex,
				Value:                    lineVal,
			})
			multiLineIndex++
		}

		buffer = ""
	}

	for _, part := range diffArray {
		if part.Added {
			buffer += part.Value
		} else {
			if buffer != "" {
				flushBuffer(part.Value)
				appendedText += buffer
			}
			originalText += part.Value
			appendedText += part.Value
		}
	}

	if buffer != "" {
		flushBuffer("")
	}

	return InlineAndFullLineDiff{
		InlineModifications:   inlineMods,
		FullLineModifications: lineMods,
	}
}

type Position struct {
	LineNumber int
	Column     int
}

func modificationsOccurBefore(diffObj InlineAndFullLineDiff, pos Position) bool {
	for _, lm := range diffObj.FullLineModifications {
		if lm.BeforeLineNumber < pos.LineNumber {
			return true
		}
	}
	for _, im := range diffObj.InlineModifications {
		if im.LineNumber == pos.LineNumber && im.Column < pos.Column {
			return true
		}
	}
	return false
}

type OffsetDiffResult struct {
	Success               bool
	InlineModifications   []InlineModification
	FullLineModifications []struct {
		BeforeLineNumber         int
		IndexInMultilineAddition int
		Content                  string
		Decorations              []GhostTextDecoration
	}
}

func ApplyDiffWithOffset(
	diffArray []DiffPart,
	lineOffset int,
	pos Position,
	param1, param2 any,
) OffsetDiffResult {
	iafl := computeInlineAndFullLineDiff(diffArray)

	shiftedLineMods := make([]struct {
		BeforeLineNumber         int
		IndexInMultilineAddition int
		Content                  string
		Decorations              []GhostTextDecoration
	}, 0, len(iafl.FullLineModifications))

	for _, lm := range iafl.FullLineModifications {
		dec := GhostTextDecoration{
			StartColumn: 1,
			EndColumn:   len(lm.Value) + 1,
			ClassName:   "ghost-text",
			SomeFlag:    0,
		}
		shiftedLineMods = append(shiftedLineMods, struct {
			BeforeLineNumber         int
			IndexInMultilineAddition int
			Content                  string
			Decorations              []GhostTextDecoration
		}{
			BeforeLineNumber:         lm.BeforeLineNumber + lineOffset,
			IndexInMultilineAddition: lm.IndexInMultilineAddition,
			Content:                  lm.Value,
			Decorations:              []GhostTextDecoration{dec},
		})
	}

	shiftedInlineMods := make([]InlineModification, 0, len(iafl.InlineModifications))
	for _, im := range iafl.InlineModifications {
		shiftedInlineMods = append(shiftedInlineMods, InlineModification{
			LineNumber: im.LineNumber + lineOffset,
			Column:     im.Column,
			Value:      im.Value,
		})
	}

	shiftedDiffObj := InlineAndFullLineDiff{
		InlineModifications:   shiftedInlineMods,
		FullLineModifications: []FullLineModification{},
	}
	for _, slm := range shiftedLineMods {
		shiftedDiffObj.FullLineModifications = append(shiftedDiffObj.FullLineModifications,
			FullLineModification{
				BeforeLineNumber:         slm.BeforeLineNumber,
				IndexInMultilineAddition: slm.IndexInMultilineAddition,
				Value:                    slm.Content,
			},
		)
	}

	if modificationsOccurBefore(shiftedDiffObj, pos) {
		return OffsetDiffResult{
			Success:               false,
			InlineModifications:   shiftedInlineMods,
			FullLineModifications: shiftedLineMods,
		}
	} else {
		return OffsetDiffResult{
			Success:               true,
			InlineModifications:   shiftedInlineMods,
			FullLineModifications: shiftedLineMods,
		}
	}
}

type WordAndCharDiffs struct {
	WordDiffs []DiffPart
	CharDiffs []DiffPart
}

func ComputeWordAndCharDiffs(oldText, newText, delimiter string) WordAndCharDiffs {
	lineDiffs := lineDiffWithTrimming(oldText, newText, delimiter)

	var wordDiffs []DiffPart
	var charDiffs []DiffPart

	group := make([]DiffPart, 0)

	flushGroup := func(nextValue string) {
		if len(group) > 0 {
			// Combine removed and added
			var removedString, addedString string
			for _, part := range group {
				if part.Removed {
					removedString += part.Value
				} else if part.Added {
					addedString += part.Value
				}
			}
			// `uDs` might do a "word-level" diff; we use a stub
			wordResult := uDsStub(removedString, addedString, nil, false)

			// For character-level diff, we do the guarded approach
			charResult := DiffCharsGuarded(removedString, addedString, nil)

			wordDiffs = append(wordDiffs, wordResult...)
			charDiffs = append(charDiffs, charResult...)

			// Clear
			group = group[:0]
		}
	}

	for _, part := range lineDiffs {
		if part.Added || part.Removed {
			group = append(group, part)
		} else {
			flushGroup(part.Value)
			// Neutral chunk => add to both
			wordDiffs = append(wordDiffs, part)
			charDiffs = append(charDiffs, part)
		}
	}
	// Flush any leftover group
	flushGroup("")

	return WordAndCharDiffs{
		WordDiffs: wordDiffs,
		CharDiffs: charDiffs,
	}
}

// ------------------------------------------------------------------------------------
// 6. Splitting lines and removing the common prefix.

type TrimResult struct {
	TrimmedOldLines   []string
	TrimmedNewLines   []string
	StartIndex        int
	RemovedStartLines []string
}

// splitAndTrimCommonLines splits each string by `delimiter`, then skips
// the common prefix of lines. Returns the leftover lines plus what was removed.
func splitAndTrimCommonLines(oldText, newText, delimiter string) TrimResult {
	oldLines := strings.Split(oldText, delimiter)
	newLines := strings.Split(newText, delimiter)

	index := 0
	for index < len(oldLines) && index < len(newLines) && oldLines[index] == newLines[index] {
		index++
	}

	return TrimResult{
		TrimmedOldLines:   oldLines[index:],
		TrimmedNewLines:   newLines[index:],
		StartIndex:        index,
		RemovedStartLines: oldLines[:index],
	}
}

// ------------------------------------------------------------------------------------
// 7. Main line-based diff function with LCS.

func lineDiffWithTrimming(oldText, newText, delimiter string) []DiffPart {
	tr := splitAndTrimCommonLines(oldText, newText, delimiter)

	finalDiff := []DiffPart{}

	// If we found a common prefix, add it as a neutral diff block
	if tr.StartIndex > 0 {
		joined := strings.Join(tr.RemovedStartLines, delimiter)
		finalDiff = append(finalDiff, DiffPart{Value: joined})
		// If there's leftover on both sides, ensure we keep a delimiter
		if len(tr.TrimmedOldLines) > 0 && len(tr.TrimmedNewLines) > 0 {
			finalDiff[0].Value += delimiter
		} else if len(tr.TrimmedOldLines) > 0 {
			// unify them with an extra blank if needed
			tr.TrimmedOldLines = append([]string{""}, tr.TrimmedOldLines...)
		} else if len(tr.TrimmedNewLines) > 0 {
			tr.TrimmedNewLines = append([]string{""}, tr.TrimmedNewLines...)
		}
	}

	// Build LCS matrix for the leftover
	matrix := buildLcsMatrix(tr.TrimmedOldLines, tr.TrimmedNewLines)
	rawLineDiff := backtrackLcs(matrix, tr.TrimmedOldLines, tr.TrimmedNewLines)
	merged := mergeSameTypeBlocks(rawLineDiff)
	finalBlocks := fixLineBoundaries(merged)
	finalDiff = append(finalDiff, finalBlocks...)
	return finalDiff
}

// buildLcsMatrix builds a 2D matrix for LCS of arrays of lines.
func buildLcsMatrix(oldLines, newLines []string) [][]int {
	rows := len(oldLines) + 1
	cols := len(newLines) + 1
	matrix := make([][]int, rows)
	for i := 0; i < rows; i++ {
		matrix[i] = make([]int, cols)
	}

	for r := 1; r < rows; r++ {
		for c := 1; c < cols; c++ {
			if oldLines[r-1] == newLines[c-1] {
				matrix[r][c] = matrix[r-1][c-1] + 1
			} else {
				if matrix[r-1][c] > matrix[r][c-1] {
					matrix[r][c] = matrix[r-1][c]
				} else {
					matrix[r][c] = matrix[r][c-1]
				}
			}
		}
	}
	return matrix
}

// backtrackLcs traverses the LCS matrix to produce a diff array
func backtrackLcs(matrix [][]int, oldLines, newLines []string) []DiffPart {
	var result []DiffPart
	r := len(oldLines)
	c := len(newLines)

	for r > 0 || c > 0 {
		if r > 0 && c > 0 && oldLines[r-1] == newLines[c-1] {
			result = prepend(result, DiffPart{Value: oldLines[r-1]})
			r--
			c--
		} else if c > 0 && (r == 0 || matrix[r][c-1] >= matrix[r-1][c]) {
			result = prepend(result, DiffPart{Value: newLines[c-1], Added: true})
			c--
		} else if r > 0 && (c == 0 || matrix[r][c-1] < matrix[r-1][c]) {
			result = prepend(result, DiffPart{Value: oldLines[r-1], Removed: true})
			r--
		}
	}

	return result
}

// prepend helper for slices of DiffPart
func prepend(slice []DiffPart, val DiffPart) []DiffPart {
	return append([]DiffPart{val}, slice...)
}

// mergeSameTypeBlocks merges consecutive blocks of the same added/removed status
func mergeSameTypeBlocks(diffArray []DiffPart) []DiffPart {
	var merged []DiffPart
	var current *DiffPart

	for _, part := range diffArray {
		if current != nil && current.Added == part.Added && current.Removed == part.Removed {
			current.Value += "\n" + part.Value
		} else {
			if current != nil {
				merged = append(merged, *current)
			}
			newPart := part
			current = &newPart
		}
	}
	if current != nil {
		merged = append(merged, *current)
	}
	return merged
}

// fixLineBoundaries tries to ensure newlines are properly inserted.
func fixLineBoundaries(diffArray []DiffPart) []DiffPart {
	result := make([]DiffPart, len(diffArray))
	copy(result, diffArray)

	for i := 0; i < len(result)-1; i++ {
		current := result[i]
		// Find next chunk that’s not "added"
		var nextNonAdded *DiffPart
		// Find next chunk that’s not "removed"
		var nextNonRemoved *DiffPart

		for idx := i + 1; idx < len(result); idx++ {
			if nextNonAdded == nil && !result[idx].Added {
				nextNonAdded = &result[idx]
			}
			if nextNonRemoved == nil && !result[idx].Removed {
				nextNonRemoved = &result[idx]
			}
			if nextNonAdded != nil && nextNonRemoved != nil {
				break
			}
		}

		if current.Removed {
			if nextNonAdded != nil {
				result[i].Value += "\n"
			}
		} else if current.Added {
			if nextNonRemoved != nil {
				result[i].Value += "\n"
			}
		} else {
			// neutral chunk
			if nextNonAdded != nil && nextNonRemoved != nil {
				result[i].Value += "\n"
			} else if nextNonAdded != nil {
				nextNonAdded.Value = "\n" + nextNonAdded.Value
			} else if nextNonRemoved != nil {
				nextNonRemoved.Value = "\n" + nextNonRemoved.Value
			}
		}
	}

	// Filter out empty parts
	var filtered []DiffPart
	for _, p := range result {
		if p.Value != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// ------------------------------------------------------------------------------------
// 8. Utility calls from the original code, likely no-ops. You can remove or keep them.

func Z()  {}
func Te() {}
func Pe() {}
func hn() {}
func Os() {}
func pt() {}
func Ce() {}
func ht() {}
func Nn() {}
func Ss() {}
func We() {}
func xe() {}

// ------------------------------------------------------------------------------------
// 9. Example or leftover variable, plus a random string generator.

var x1r = 10

// GenerateRandomString returns a random 10-letter string from 'abcdefghijklmnopqrstuvwxyz'.
func GenerateRandomString() string {
	letters := "abcdefghijklmnopqrstuvwxyz"
	rand.Seed(time.Now().UnixNano())
	result := make([]byte, 10)
	for i := 0; i < 10; i++ {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}
