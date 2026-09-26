package workspace

import (
	"errors"
)

var (
	// 内容为空
	ErrContentEmpty = errors.New("The content must not be empty")

	// 内容与规则不符
	ErrInvalidFilename = errors.New("内容格式与正则规则不符")

	// os.MkdirAll 创建目录失败
	ErrMkdirAll = errors.New("创建目录失败")

	// os.WriteFile 写入文件失败
	ErrWriteFile = errors.New("文件写入失败")

	// ErrReadFile 读取文件时失败
	ErrReadFile = errors.New("读取文件失败")

	// ErrFileTooLarge 读取文件超过限制。
	ErrFileTooLarge = errors.New("文件内容超过允许读取的大小")

	// ErrInvalidPath 非法路径
	ErrInvalidPath = errors.New("非法路径")

	// ErrOpenWorkspace 打开工作区失败
	ErrOpenWorkspace = errors.New("Failed to open the workspace")

	// ErrUnsupportedFileType 路径下不是合法文本文件
	ErrUnsupportedFileType = errors.New("unsupported text file type")

	// ErrSymlinkPath 路径包含符号链接
	ErrSymlinkPath = errors.New("symbolic links are not allowed")
)
