package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

// WriteTextFileTool 经用户确认后，在受控工作区中创建或覆盖文本文件。
type WriteTextFileTool struct {
	workspace *Workspace
	confirmer tool.Confirmer
}

// NewWriteTextFileTool 创建使用正式工作区的文本写入工具。
func NewWriteTextFileTool(workspace *Workspace, confirmer tool.Confirmer) (*WriteTextFileTool, error) {
	if workspace == nil || workspace.root == nil {
		return nil, fmt.Errorf("create write_text_file tool: workspace is nil")
	}

	if confirmer == nil {
		return nil, fmt.Errorf(
			"create write_text_file tool: confirmer is nil",
		)
	}

	return &WriteTextFileTool{
		workspace: workspace,
		confirmer: confirmer,
	}, nil
}

type writeTextFileArguments struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

var writeTextFileParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Workspace-relative destination path using '/' separators, including nested paths such as internal/config/config.go. Do not use an absolute path."
    },
    "content": {
      "type": "string",
      "description": "The complete final text content for the file. Existing content is fully replaced after host-side user approval; this value is not a patch, diff, or append operation."
    }
  },
  "required": ["path", "content"],
  "additionalProperties": false
}`)

func (t *WriteTextFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "write_text_file",
		Description: "Propose creating or completely replacing one supported regular text file in the " +
			"controlled workspace. Provide a workspace-relative path using '/' separators; nested paths " +
			"such as internal/config/config.go are supported, while absolute paths and symbolic-link " +
			"paths are rejected. The content argument is the complete desired file content, not a patch " +
			"or partial edit. Calling this tool does not bypass confirmation: after the call, the host " +
			"application displays the proposed path and complete content to the user and waits for an " +
			"explicit decision. The file is modified only if the host reports approval; rejection causes " +
			"no filesystem change. Do not ask for a second confirmation immediately before calling this " +
			"tool unless the user's intent to modify a file is itself unclear.",
		Parameters: writeTextFileParameters,
	}
}

func (t *WriteTextFileTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	args, err := tool.DecodeObjectArguments[writeTextFileArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse write_text_file arguments: %w",
			err,
		)
	}

	toolPath, err := t.workspace.validateTextFileWrite(args.Path, args.Content)
	if err != nil {
		return "", fmt.Errorf(
			"validate write_text_file arguments: %w",
			err,
		)
	}

	confirmed, err := t.confirmer.Confirm(
		ctx,
		tool.ConfirmationRequest{
			Action:  "write_text_file",
			Summary: fmt.Sprintf("创建或覆盖文件 %s", toolPath),
			Details: fmt.Sprintf(
				"内容长度: %d 字符, %d 字节\n\n内容:\n%s",
				utf8.RuneCountInString(args.Content),
				len(args.Content),
				args.Content,
			),
		},
	)

	if err != nil {
		return "", fmt.Errorf("Authorization write text file operation failed: %w", err)
	}

	if !confirmed {
		return fmt.Sprintf(
			"The write file %s operation was rejected, No modifications were made",
			toolPath,
		), nil
	}

	if err = t.workspace.WriteTextFile(toolPath, args.Content); err != nil {
		return "", fmt.Errorf(
			"write text file %q: %w",
			toolPath,
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before result returned: %w", err)
	}

	return fmt.Sprintf("File %q written successfully", toolPath), nil
}

var _ tool.Tool = (*WriteTextFileTool)(nil)
