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
		Description: "Recursively list supported regular text files in the controlled workspace. " +
			"The workspace root and absolute filesystem location are intentionally hidden. Every " +
			"returned value is a workspace-relative path using '/' separators, and nested files may " +
			"appear as paths such as internal/config/config.go. Supported files include go.mod, go.sum, " +
			"and files ending in .go, .md, .txt, .json, .yaml, .yml, or .toml. Symbolic links and " +
			"non-regular files are excluded. If truncated is true, the listing is incomplete because " +
			"a safety limit was reached; this tool currently has no pagination or continuation cursor.",
		Parameters: listTextFilesParameters,
	}
}

type listTextFilesResponse struct {
	Paths     []string `json:"paths"`
	Truncated bool     `json:"truncated"`
}

func (l *ListTextFilesTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	_, err := tool.DecodeObjectArguments[struct{}](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse list_text_files arguments: %w",
			err,
		)
	}

	fileList, err := l.workspace.ListTextFiles(ctx)
	if err != nil {
		return "", fmt.Errorf("execute ListTextFiles(ctx): %w", err)
	}

	response := listTextFilesResponse{
		Paths:     fileList.Paths,
		Truncated: fileList.Truncated,
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("marshal ListTextFiles(ctx).got: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before result returned: %w", err)
	}

	return string(encoded), nil
}

var _ tool.Tool = (*ListTextFilesTool)(nil)
