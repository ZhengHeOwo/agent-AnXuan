package workspace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type searchIssueKind string

const (
	issueOpenFailed         searchIssueKind = "open_failed"
	issueNotRegular         searchIssueKind = "not_regular_file"
	issueFileTooLarge       searchIssueKind = "file_too_large"
	issueLineTooLong        searchIssueKind = "line_too_long"
	issueReadFailed         searchIssueKind = "read_failed"
	issueVerificationFailed searchIssueKind = "verification_failed"
)

type searchIssue struct {
	Path           string
	Kind           searchIssueKind
	Err            error
	InfiniteAmount bool
}

const maxIssueTextsPerKind = 3

type issueCollector struct {
	kept   []searchIssue
	counts map[searchIssueKind]int
}

func newIssueCollector() *issueCollector {
	return &issueCollector{counts: map[searchIssueKind]int{}}
}

func (c *issueCollector) add(issue *searchIssue) {
	c.counts[issue.Kind]++
	if issue.InfiniteAmount {
		c.kept = append(c.kept, *issue)
		return
	}

	if c.counts[issue.Kind] <= maxIssueTextsPerKind {
		c.kept = append(c.kept, *issue)
	}
}

func (c *issueCollector) Error() string {
	return searchIssueToolResultResponse(c)
}

var _ error = (*issueCollector)(nil)

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
) ([]TextMatch, bool, *searchIssue) {
	if err := ctx.Err(); err != nil {
		// 取消或超时导致该文件没有被搜索，按未完整扫描处理，错误由 SearchText 统一返回。
		return nil, true, nil
	}

	if err := validateSearchQuery(query); err != nil {
		return nil, false, &searchIssue{
			Path: input,
			Kind: issueVerificationFailed,
			Err:  err,
		}
	}

	toolPath, err := validateToolPath(input)
	if err != nil {
		return nil, false, &searchIssue{
			Path: input,
			Kind: issueVerificationFailed,
			Err:  err,
		}
	}

	if !isAllowedTextFile(toolPath) {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueVerificationFailed,
			Err:  ErrUnsupportedFileType,
		}
	}

	localPath, err := localizeToolPath(toolPath)
	if err != nil {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueReadFailed,
			Err:  err,
		}
	}

	if err := w.rejectSymlinkPath(localPath); err != nil {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueReadFailed,
			Err:  err,
		}
	}

	file, err := w.root.Open(localPath)
	if err != nil {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueOpenFailed,
			Err:  err,
		}
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueReadFailed,
			Err:  err,
		}
	}

	if !info.Mode().IsRegular() {
		return nil, false, &searchIssue{
			Path: toolPath,
			Kind: issueNotRegular,
			Err:  nil,
		}
	}

	if info.Size() > maxSearchFileBytes {
		return []TextMatch{}, true, &searchIssue{
			Path:           toolPath,
			Kind:           issueFileTooLarge,
			Err:            nil,
			InfiniteAmount: true,
		}
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
			// 同上：已收集的命中照常返回，剩余内容不再扫描。
			return matches, true, nil
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
		kind := issueReadFailed
		if errors.Is(err, bufio.ErrTooLong) {
			kind = issueLineTooLong
		}
		return matches, true, &searchIssue{
			Path: toolPath,
			Kind: kind,
			Err:  err,
		}
	}

	return matches, false, nil
}

type searchFileJob struct {
	Index int
	Path  string
}

type searchFileResult struct {
	Index       int
	Matches     []TextMatch
	Truncated   bool
	SearchIssue *searchIssue
}

type TextSearchResult struct {
	Matches                []TextMatch
	SearchTruncated        bool
	CandidateListTruncated bool
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

			matches, truncated, searchIssue := w.searchTextFile(
				ctx,
				job.Path,
				query,
			)

			result := searchFileResult{
				Index:       job.Index,
				Matches:     matches,
				Truncated:   truncated,
				SearchIssue: searchIssue,
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
) (*TextSearchResult, error) {
	result := &TextSearchResult{
		Matches: make([]TextMatch, 0),
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validateSearchQuery(query); err != nil {
		return nil, err
	}

	fileList, err := w.ListTextFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"通过使用 list_text_files 底层实现获取工作区可搜索文件列表失败, 原因: %w",
			err,
		)
	}

	result.CandidateListTruncated = fileList.truncated

	if len(fileList.paths) == 0 {
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

		for index, path := range fileList.paths {
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
		len(fileList.paths),
	)

	issueCollector := newIssueCollector()
	for fileResult := range results {
		if fileResult.Truncated {
			result.SearchTruncated = true
		}

		if fileResult.SearchIssue != nil {
			issueCollector.add(fileResult.SearchIssue)
		}

		fileResults[fileResult.Index] = fileResult
	}

collectMatches:
	for _, fileResult := range fileResults {
		for _, match := range fileResult.Matches {
			if len(result.Matches) >= maxSearchMatches {
				result.SearchTruncated = true
				break collectMatches
			}

			result.Matches = append(
				result.Matches,
				match,
			)
		}
	}

	if err := ctx.Err(); err != nil {
		result.SearchTruncated = true
		return result, err
	}

	if len(issueCollector.kept) != 0 {
		return result, issueCollector
	}

	return result, nil
}
