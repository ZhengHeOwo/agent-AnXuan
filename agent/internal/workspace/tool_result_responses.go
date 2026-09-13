package workspace

import (
	"fmt"
)

// read_text_file_tool 结果包装
func readTextFileToolResultResponse(result *readTextFileResult) string {
	if result == nil {
		return ""
	}
	return fmt.Sprintf(
		"内容是否被截断: %t | bytes: %d | runes: %d | lines: %d\n正文:\n%s",
		result.truncated,
		result.bytes,
		result.runes,
		result.lines,
		result.content,
	)
}

// list_text_files_tool 结果包装
func listTextFilesToolResultResponse(result *textFileList) string {
	if result == nil {
		return ""
	}

	return fmt.Sprintf(
		"列表是否被截断: %t |\n列表结果:\n%q",
		result.truncated,
		result.paths,
	)
}
