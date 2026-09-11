package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
)

type TextMatch struct {
	Path          string
	Line          int
	Text          string
	TextTruncated bool
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

	if err := validateSearchQuery(query); err != nil {
		return nil, false, err
	}

	toolPath, err := validateToolPath(input)
	if err != nil {
		return nil, false, fmt.Errorf(
			"%q未通过验证, 因为: %w",
			input,
			err,
		)
	}

	if !isAllowedTextFile(toolPath) {
		return nil, false, fmt.Errorf(
			"%q 未通过验证, 因为: %w",
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
			"打开 %q 失败, 因为: %w",
			toolPath,
			err,
		)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf(
			"获取 %q 信息失败, 因为: %w",
			toolPath,
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf(
			"%q 不是普通文件",
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

		text, textTruncated := makeMatchText(
			line,
			queryBytes,
		)

		matches = append(matches, TextMatch{
			Path:          toolPath,
			Line:          lineNumber,
			Text:          text,
			TextTruncated: textTruncated,
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

type searchFileJob struct {
	Index int
	Path  string
}

type searchFileResult struct {
	Index     int
	Matches   []TextMatch
	Truncated bool
	Err       error
}

type TextSearchResult struct {
	Matches   []TextMatch
	Truncated bool
}

const (
	searchWorkerCount = 4
	maxSearchMatches  = 100
)

const (
	maxSearchQueryBytes     = 1 << 10
	maxStoredMatchTextBytes = 2 << 10
)

func validateSearchQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf(
			"搜索目标为空",
		)
	}

	if len(query) > maxSearchQueryBytes {
		return fmt.Errorf(
			"搜索目标过大(上限 %d 字节, 当前目标 %d 字节)",
			maxSearchQueryBytes,
			len(query),
		)
	}

	return nil
}

func makeMatchText(
	line []byte,
	query []byte,
) (string, bool) {
	lineLength := len(line)
	if lineLength <= maxStoredMatchTextBytes {
		return string(line), false
	}

	matchStart := bytes.Index(line, query)
	if matchStart < 0 {
		return "", false
	}

	availableContent := maxStoredMatchTextBytes - len(query)
	start := matchStart - availableContent/2

	if start < 0 {
		start = 0
	}

	end := start + maxStoredMatchTextBytes
	if end > lineLength {
		end = lineLength

		start = end - maxStoredMatchTextBytes
		if start < 0 {
			start = 0
		}
	}

	text := strings.ToValidUTF8(
		string(line[start:end]),
		"\uFFFD",
	)

	if start > 0 {
		text = "..." + text
	}

	if end < lineLength {
		text += "..."
	}

	return text, true
}

func (w *Workspace) searchTextWorker(
	ctx context.Context,
	query string,
	jobs <-chan searchFileJob,
	results chan<- searchFileResult,
) {
	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-jobs:
			if !ok {
				return
			}

			matches, truncated, err := w.searchTextFile(
				ctx,
				job.Path,
				query,
			)

			result := searchFileResult{
				Index:     job.Index,
				Matches:   matches,
				Truncated: truncated,
				Err:       err,
			}

			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}

	}
}

func (w *Workspace) SearchText(
	ctx context.Context,
	query string,
) (TextSearchResult, error) {
	result := TextSearchResult{
		Matches: make([]TextMatch, 0),
	}

	if err := ctx.Err(); err != nil {
		return TextSearchResult{}, err
	}

	if err := validateSearchQuery(query); err != nil {
		return TextSearchResult{}, err
	}

	fileList, err := w.ListTextFiles(ctx)
	if err != nil {
		return TextSearchResult{}, fmt.Errorf(
			"获取工作区可搜索文件列表失败, 因为: %w",
			err,
		)
	}

	result.Truncated = fileList.Truncated

	if len(fileList.Paths) == 0 {
		return result, nil
	}

	searchContext, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan searchFileJob)
	results := make(chan searchFileResult)

	var workers sync.WaitGroup
	workers.Add(searchWorkerCount)

	for range searchWorkerCount {
		go func() {
			defer workers.Done()

			w.searchTextWorker(
				searchContext,
				query,
				jobs,
				results,
			)
		}()

	}

	go func() {
		defer close(jobs)

		for index, path := range fileList.Paths {
			job := searchFileJob{
				Index: index,
				Path:  path,
			}

			select {
			case jobs <- job:
			case <-searchContext.Done():
				return
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	fileResults := make(
		[]searchFileResult,
		len(fileList.Paths),
	)

	var firstError error
	for fileResult := range results {
		if fileResult.Err != nil {
			if firstError == nil {
				firstError = fileResult.Err
				cancel()
			}

			continue
		}

		fileResults[fileResult.Index] = fileResult
	}

	if firstError != nil {
		return TextSearchResult{}, fmt.Errorf(
			"搜索工作区文本失败, 因为: %w",
			firstError,
		)
	}

	if err := ctx.Err(); err != nil {
		return TextSearchResult{}, err
	}

	for _, fileResult := range fileResults {
		if fileResult.Truncated {
			result.Truncated = true
		}

		for _, match := range fileResult.Matches {
			if len(result.Matches) >= maxSearchMatches {
				result.Truncated = true
				return result, nil
			}

			result.Matches = append(
				result.Matches,
				match,
			)
		}
	}

	return result, nil
}
