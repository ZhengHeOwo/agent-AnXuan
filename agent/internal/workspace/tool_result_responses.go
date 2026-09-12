package workspace

import (
	"fmt"
)

// read_text_file_tool 结果包装
func readTextFileToolResultResponse(result *readTextFileResult) string {
	return fmt.Sprintf(
		"内容是否截断: %t | bytes: %d | runes: %d | lines: %d\n正文:\n%s",
		result.truncated,
		result.bytes,
		result.runes,
		result.lines,
		result.content,
	)
}
