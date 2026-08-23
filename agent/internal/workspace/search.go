package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
)

type TextMatch struct {
	Path string
	Line int
	Text string
}

const (
	maxSearchFileBytes       = 8 << 20
	initialSearchBufferBytes = 64 << 10
	maxSearchLineBytes       = 256 << 10
	maxMatchesPerFile        = 20
)

func (w *Workspace) searchTextFile(
	ctx context.Context,
	input string,
	query string,
) ([]TextMatch, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	if strings.TrimSpace(query) == "" {
		return nil, false, fmt.Errorf(
			"search query must not be empty",
		)
	}

	toolPath, err := validateToolPath(input)
	if err != nil {
		return nil, false, fmt.Errorf(
			"validate search path %q: %w",
			input,
			err,
		)
	}

	if !isAllowedTextFile(toolPath) {
		return nil, false, fmt.Errorf(
			"validate search path %q: %w",
			toolPath,
			ErrUnsupportedFileType,
		)
	}

	localPath, err := localizeToolPath(toolPath)
	if err != nil {
		return nil, false, err
	}

	if err := w.rejectSymlinkPath(localPath); err != nil {
		return nil, false, err
	}

	file, err := w.root.Open(localPath)
	if err != nil {
		return nil, false, fmt.Errorf(
			"open search file %q: %w",
			toolPath,
			err,
		)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf(
			"stat search file %q: %w",
			toolPath,
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf(
			"search file %q is not a regular file",
			toolPath,
		)
	}

	if info.Size() > maxSearchFileBytes {
		return []TextMatch{}, true, nil
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(
		make([]byte, initialSearchBufferBytes),
		maxSearchLineBytes,
	)

	queryBytes := []byte(query)
	matches := make([]TextMatch, 0)
	lineNumber := 0

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}

		lineNumber++
		line := scanner.Bytes()

		if !bytes.Contains(line, queryBytes) {
			continue
		}

		if len(matches) >= maxMatchesPerFile {
			return matches, true, nil
		}

		matches = append(matches, TextMatch{
			Path: toolPath,
			Line: lineNumber,
			Text: string(line),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf(
			"scan text file %q: %w",
			toolPath,
			err,
		)
	}

	return matches, false, nil
}
