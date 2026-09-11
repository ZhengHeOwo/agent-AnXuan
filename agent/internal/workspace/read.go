package workspace

import (
	"fmt"
	"io"
)

const maxReadBytes = 400_000

func (w *Workspace) ReadTextFile(input string) (string, error) {
	toolPath, err := validateToolPath(input)
	if err != nil {
		return "", err
	}

	if !isAllowedTextFile(toolPath) {
		return "", fmt.Errorf("%v 被拒绝, 因为: %w", toolPath, ErrUnsupportedFileType)
	}

	localPath, err := localizeToolPath(toolPath)
	if err != nil {
		return "", err
	}

	if err = w.rejectSymlinkPath(localPath); err != nil {
		return "", err
	}

	file, err := w.root.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("打开 %v 失败, 因为: %w", localPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取 %v 信息失败, 因为: %w", toolPath, err)
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%v被拒绝, 因为: 不是普通文件", toolPath)
	}

	reader := io.LimitReader(file, int64(maxReadBytes+1))

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf(
			"读取 %v 失败: %w(上限 %d 字节, 本次未返回文件内容)",
			toolPath,
			err,
			maxReadBytes,
		)
	}

	if len(data) > maxReadBytes {
		return "", fmt.Errorf("读取 %v 失败, 因为: %w", toolPath, ErrFileTooLarge)
	}

	return string(data), nil
}
