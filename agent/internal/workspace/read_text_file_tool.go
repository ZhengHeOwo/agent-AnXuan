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
		Description: `读取受控工作区内的一个受支持的普通文本文件。路径必须是工作区相对路径,使用 '/' 分隔,可以包含多层目录;工作区根的位置以及它在磁盘上的绝对位置不会暴露。绝对路径、不受支持的文件类型、非普通文件(目录、设备、管道、套接字等)以及符号链接路径都会被拒绝。不确定文件是否存在时,先用 search_text 查找。

成功读取时返回 content,它是文件正文按行编号后的渲染结果:正文的每一行对应输出中的一行,该行前面加上从 1 开始的行号、' | ' 和这一行的文本,行号按本次输出最大行号的位数右对齐,每行末尾的 '\r' 会被去掉,并且每行都以换行结束。例如宽度为 3 时形如 "  7 | package workspace"。content 不是文件正文本身,行号和分隔符都是附加的。

bytes 和 runes 描述的是编号之前的文件正文,分别是它的字节数和字符数(Unicode 码点数),不是 content 的字节数和字符数,所以 bytes 会小于 content 的实际长度。

lines 是整个文件的行数,由一次独立的全文扫描得出,因此可能大于 content 里实际出现的行数。空文件是 0 行,末尾没有换行符的最后一行同样计入。

读取的正文字节上限是 1000000。文件更大时只读取前 1000000 字节;如果这个上限正好落在某个多字节字符的中间,这段不完整的字节会被去掉,所以 content 的最后一行可能是不完整的一行。这种情况 truncated 为 true,否则为 false。

任何失败,包括路径被拒绝、文件类型不受支持、不是普通文件、读取过程中出错,都直接返回错误,不返回部分内容。`,
		Parameters: readTextFileParameters,
	}
}

func (r *ReadTextFileTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 read_text_file 工具前失败, 因为: %w", err)
	}

	args, err := tool.DecodeObjectArguments[readTextFileArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"解析 parse read_text_file 参数失败, 因为: %w",
			err,
		)
	}

	result, err := r.workspace.ReadTextFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("使用 read_text_file 工具获取文件 %q 内容失败, 因为: %w", args.Path, err)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 read_text_file 工具后, 即将返回结果时失败, 因为: %w", err)
	}

	return readTextFileToolResultResponse(result), nil
}

var _ tool.Tool = (*ReadTextFileTool)(nil)
