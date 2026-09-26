package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

// ListTextFilesTool 列出受控工作区中的可读取文本文件。
type ListTextFilesTool struct {
	workspace *Workspace
}

// NewListTextFilesTool 创建使用正式工作区的文件列表工具。
func NewListTextFilesTool(workspace *Workspace) (*ListTextFilesTool, error) {
	if workspace == nil || workspace.root == nil {
		return nil, fmt.Errorf("create list_text_files tool: workspace is nil")
	}

	return &ListTextFilesTool{
		workspace: workspace,
	}, nil
}

var listTextFilesParameters = json.RawMessage(`{
  "type": "object",
  "properties": {},
  "additionalProperties": false
}`)

func (l *ListTextFilesTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "list_text_files",
		Description: "递归列出受控工作区内受支持的普通文本文件。\n" +
			"工作区真实根目录和绝对路径不会暴露：返回的每个值都是工作区相对路径，以 '/' 分隔，嵌套文件形如 internal/config/config.go。\n" +
			"结果只包含受支持的普通文本文件，即文件名恰为 go.mod 或 go.sum，以及扩展名恰为 .go、.md、.txt、.json、.yaml、.yml、.toml（区分大小写，只匹配小写）的文件；目录、符号链接和其他非普通文件不会被列出，不受支持的文件会被跳过。\n" +
			"本工具不需要参数。返回的不是 JSON，而是纯文本：第一行是 \"列表是否被截断: true |\" 或 \"列表是否被截断: false |\"，第二行是 \"列表结果:\"，第三行是路径列表，形如 [\"go.mod\" \"main.go\"]，其中每个路径是 Go 字符串字面量，用双引号包裹、以空格分隔。顺序为目录深度优先遍历，同一目录内按名称排序。\n" +
			"结果被标为截断有两种原因：遍历到的条目（含目录和符号链接）超过 2000 个，或已列出的文件达到 1500 个。被截断时列表不完整，缺少遍历顺序靠后的部分；本工具没有分页或续读游标，无法取得剩余内容，应改用更精确的工具定位文件。\n" +
			"遍历失败（如某个条目无法访问）时工具直接报错，返回内容为空，不包含此前已经收集到的路径。",
		Parameters: listTextFilesParameters,
	}
}

type listTextFilesResponse struct {
	Paths     []string `json:"paths"`
	Truncated bool     `json:"truncated"`
}

func (l *ListTextFilesTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 list_text_files 工具前失败, 因为: %w", err)
	}

	_, err := tool.DecodeObjectArguments[struct{}](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"解析 list_text_files 参数失败, 因为: %w",
			err,
		)
	}

	executeResult, err := l.workspace.ListTextFiles(ctx)
	result := listTextFilesToolResultResponse(executeResult)

	if err != nil {
		return result, fmt.Errorf("使用 list_text_files 获取文件列表失败, 因为: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("执行 list_text_files 工具后, 即将返回结果时失败, 因为: %w", err)
	}

	return result, nil
}

var _ tool.Tool = (*ListTextFilesTool)(nil)
