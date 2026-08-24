package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

// ReadTextFileTool 读取受控工作区中的指定文本文件。
type ReadTextFileTool struct {
	workspace *Workspace
}

// NewReadTextFileTool 创建使用正式工作区的文本读取工具。
func NewReadTextFileTool(workspace *Workspace) (*ReadTextFileTool, error) {
	if workspace == nil || workspace.root == nil {
		return nil, fmt.Errorf("create read_text_file tool: workspace is nil")
	}

	return &ReadTextFileTool{
		workspace: workspace,
	}, nil
}

type readTextFileArguments struct {
	Path string `json:"path"`
}

var readTextFileParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Workspace-relative path using '/' separators, including nested paths such as internal/config/config.go. Do not use an absolute path."
    }
  },
  "required": ["path"],
  "additionalProperties": false
}`)

func (r *ReadTextFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "read_text_file",
		Description: "Read the complete raw content of one supported regular text file from the " +
			"controlled workspace. The workspace root and its absolute filesystem location are " +
			"intentionally hidden; provide only a workspace-relative path, which may include nested " +
			"directories and must use '/' separators. The response contains raw file content without " +
			"line numbers. Use search_text first when the file is unknown. Files that exceed the read " +
			"size limit, unsupported file types, non-regular files, and symbolic-link paths are rejected.",
		Parameters: readTextFileParameters,
	}
}

func (r *ReadTextFileTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	args, err := tool.DecodeObjectArguments[readTextFileArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse read_text_file arguments: %w",
			err,
		)
	}

	content, err := r.workspace.ReadTextFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("read text file %q: %w", args.Path, err)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before result returned: %w", err)
	}

	return content, nil
}

var _ tool.Tool = (*ReadTextFileTool)(nil)
