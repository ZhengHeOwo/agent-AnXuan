package workspace

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxReadBytes = 1_000_000

type readTextFileResult struct {
	content   string
	bytes     int
	runes     int
	lines     int
	truncated bool
}

func (w *Workspace) ReadTextFile(input string) (*readTextFileResult, error) {
	toolPath, err := validateToolPath(input)
	if err != nil {
		return nil, err
	}

	if !isAllowedTextFile(toolPath) {
		return nil, fmt.Errorf("%v 被拒绝, 因为: %w", toolPath, ErrUnsupportedFileType)
	}

	localPath, err := localizeToolPath(toolPath)
	if err != nil {
		return nil, err
	}

	if err = w.rejectSymlinkPath(localPath); err != nil {
		return nil, err
	}

	file, err := w.root.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("打开 %v 失败, 因为: %w", toolPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取 %v 信息失败, 因为: %w", toolPath, err)
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%v被拒绝, 因为: 不是普通文件", toolPath)
	}

	reader := io.LimitReader(file, int64(maxReadBytes+1))

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf(
			"读取 %v 失败: %w(上限 %d 字节, 本次未返回文件内容)",
			toolPath,
			err,
			maxReadBytes,
		)
	}

	truncated := false

	if int64(len(data)) > maxReadBytes {
		data = trimIncompleteTrailingRune(data[:maxReadBytes])
		truncated = true
	}

	text := string(data)
	lines := strings.Split(text, "\n")

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var sb strings.Builder

	width := len(strconv.Itoa(len(lines)))

	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		fmt.Fprintf(&sb, "%*d | %s\n", width, i+1, line)
	}

	scanner := bufio.NewScanner(io.NewSectionReader(file, 0, info.Size()))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	countLines := 0
	for scanner.Scan() {
		countLines++
	}

	runes := utf8.RuneCountInString(text)
	bytes := len(data)

	if err = scanner.Err(); err != nil {
		return &readTextFileResult{
				content:   sb.String(),
				bytes:     bytes,
				runes:     runes,
				lines:     0,
				truncated: truncated,
			}, fmt.Errorf(
				"获取 %v 总行数失败, 因为: %w(已返回读取内容, 并将lines设为0)",
				toolPath,
				err,
			)
	}
	return &readTextFileResult{
		content:   sb.String(),
		bytes:     bytes,
		runes:     runes,
		lines:     countLines,
		truncated: truncated,
	}, nil
}

func trimIncompleteTrailingRune(data []byte) []byte {
	for len(data) > 0 {
		r, size := utf8.DecodeLastRune(data)
		if r != utf8.RuneError || size > 1 {
			break
		}
		data = data[:len(data)-1]
	}
	return data
}
