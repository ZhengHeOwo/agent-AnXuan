package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type ByteTooLongError struct {
	limit  int
	actual int
}

func (t *ByteTooLongError) Error() string {
	return fmt.Sprintf(
		"内容超出字节限制: 上限 %d, 当前 %d",
		t.limit,
		t.actual,
	)
}

type RuneTooLongError struct {
	limit  int
	actual int
}

func (r *RuneTooLongError) Error() string {
	return fmt.Sprintf(
		"内容超出字符限制: 上限 %d, 当前 %d",
		r.limit,
		r.actual,
	)
}

// 写入上限使用十进制口径: 1 GB = 1_000_000_000 字节。
// 码点上限取同一数值: 任何内容的码点数都不会超过它的字节数,
// 因此这道码点上限不会比字节上限更早生效, 保留它只为维持原有的两道检查结构。
const (
	maxWriteRunes = 1_000_000_000
	maxWriteBytes = 1_000_000_000
)

func validateTextFileContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return ErrContentEmpty
	}

	if characterCount := utf8.RuneCountInString(content); characterCount > maxWriteRunes {
		return &RuneTooLongError{
			limit:  maxWriteRunes,
			actual: characterCount,
		}
	}

	if len(content) > maxWriteBytes {
		return &ByteTooLongError{
			limit:  maxWriteBytes,
			actual: len(content),
		}
	}

	return nil
}

func (w *Workspace) inspectWriteTarget(
	localPath string,
) (os.FileMode, error) {
	info, err := w.root.Lstat(localPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0o644, nil
		}

		return 0, fmt.Errorf(
			"获取 %q 信息失败, 因为: %w",
			localPath,
			err,
		)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf(
			"%q 未经过校验, 因为: %w",
			localPath,
			ErrSymlinkPath,
		)
	}

	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf(
			"%q 不是普通文件",
			localPath,
		)
	}

	return info.Mode().Perm(), nil
}

func randomTemporarySuffix() (string, error) {
	var data [8]byte

	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf(
			"生成临时文件后缀失败, 因为: %w",
			err,
		)
	}

	return hex.EncodeToString(data[:]), nil
}

func (w *Workspace) createTemporaryTextFile(
	targetPath string,
	perm os.FileMode,
) (*os.File, string, error) {
	parentDir := filepath.Dir(targetPath)
	baseName := filepath.Base(targetPath)

	for range 8 {
		suffix, err := randomTemporarySuffix()
		if err != nil {
			return nil, "", err
		}

		tempName := "." + baseName + ".tmp-" + suffix
		tempPath := filepath.Join(parentDir, tempName)

		file, err := w.root.OpenFile(
			tempPath,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			perm,
		)

		if err == nil {
			return file, tempPath, nil
		}

		if !errors.Is(err, fs.ErrExist) {
			return nil, "", fmt.Errorf(
				"创建临时文件 %q 失败, 原因: %w",
				tempPath,
				err,
			)
		}
	}

	return nil, "", fmt.Errorf(
		"终止为 %q 创建临时文件, 原因: 名称冲突次数超过上限",
		targetPath,
	)
}

func (w *Workspace) replaceTextFile(
	localPath string,
	content string,
	perm os.FileMode,
) error {
	file, tempPath, err := w.createTemporaryTextFile(localPath, perm)
	if err != nil {
		return fmt.Errorf(
			"为写入操作准备临时文件失败, 因为: %w",
			err,
		)
	}

	fileClosed := false
	renamed := false
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}

		if !renamed {
			_ = w.root.Remove(tempPath)
		}
	}()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf(
			"写入临时文件 %q 失败, 原因: %w",
			tempPath,
			err,
		)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf(
			"同步临时文件 %q失败, 原因: %w",
			tempPath,
			err,
		)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf(
			"关闭临时文件 %q 失败 原因: %w",
			tempPath,
			err,
		)
	}
	fileClosed = true

	if err := w.root.Rename(tempPath, localPath); err != nil {
		return fmt.Errorf(
			"重命名文件 %q 失败, 原因: %w",
			tempPath,
			err,
		)
	}

	renamed = true
	return nil
}

func (w *Workspace) validateTextFileWrite(
	input string,
	content string,
) (string, error) {
	toolPath, err := validateToolPath(input)
	if err != nil {
		return "", fmt.Errorf(
			"写入 %q 未通过校验, 因为: %w",
			input,
			err,
		)
	}

	if !isAllowedTextFile(toolPath) {
		return "", fmt.Errorf(
			"写入 %q 未通过校验, 因为: %w",
			toolPath,
			ErrUnsupportedFileType,
		)
	}

	if err := validateTextFileContent(content); err != nil {
		return "", fmt.Errorf(
			"写入内容未通过校验, 因为: %w",
			err,
		)
	}

	return toolPath, nil
}

// writeTextFileResult 是 write_text_file_tool 需要渲染的结果:
// written 为 true 表示内容已写入目标, 为 false 表示狰和拒绝、未进行任何更改。
type writeTextFileResult struct {
	toolPath string
	written  bool
}

func (w *Workspace) WriteTextFile(
	input string,
	content string,
) (*writeTextFileResult, error) {
	toolPath, err := w.validateTextFileWrite(input, content)
	if err != nil {
		return nil, err
	}

	localPath, err := localizeToolPath(toolPath)
	if err != nil {
		return nil, err
	}

	parentDir := filepath.Dir(localPath)
	if parentDir != "." {
		if err := w.root.MkdirAll(parentDir, 0o755); err != nil {
			return nil, fmt.Errorf(
				"创建父级目录 %q 失败, 因为: %w",
				parentDir,
				err,
			)
		}

		if err := w.rejectSymlinkPath(parentDir); err != nil {
			return nil, err
		}
	}

	perm, err := w.inspectWriteTarget(localPath)
	if err != nil {
		return nil, err
	}

	if err := w.replaceTextFile(
		localPath,
		content,
		perm,
	); err != nil {
		return nil, err
	}

	return &writeTextFileResult{
		toolPath: toolPath,
		written:  true,
	}, nil
}
