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

成功读取时返回 content,它是被读取那部分正文按行编号后的渲染结果:正文的每一行对应输出中的一行,行首加上从 1 开始的行号、' | ' 和该行文本,行号按 content 中最大行号的位数右对齐,行尾的一个 '\r' 会被去掉,每行都以换行结束。例如行号宽度为 3 时形如 "  7 | package workspace"。content 不同于原正文:行号和分隔符是附加的,行尾的 '\r' 已被去掉。

bytes 和 runes 是编号之前那部分正文的字节数和字符数(Unicode 码点数),按原始字节统计,保留 '\r' 和换行符,不等于 content 的字节数或字符数。文件未被截断时它们就是整个文件的量;truncated 为 true 时只覆盖本次实际读取并渲染的那部分正文,此时末尾残缺字符的字节已被丢弃,所以 bytes 可能略小于 1000000,不能用它判断文件总大小。

lines 是整个文件的行数,由一次独立的全文扫描得出;未截断时它等于 content 的行数,截断时会大于 content 中实际出现的行数。空文件是 0 行,末尾没有换行符的最后一行同样计入。

读取的正文字节上限是 1000000。文件更大时只读取前 1000000 字节,并丢弃被这个上限切断的多字节字符的残缺字节;这些字节不会出现在 content 里,所以 content 的最后一行是原文件对应行的前半段。此时 truncated 为 true,否则为 false。文件中如果有单行超过 10MB,读取会整体失败而不返回内容。

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
