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
		Description: `读取受控工作区内的一个文本文件。path 必须是工作区相对路径,使用 '/' 分隔,可以包含多层目录。工作区根的位置以及它在磁盘上的绝对位置不会暴露。绝对路径、扩展名不在支持范围内的文件、目录以及设备、管道、套接字等非普通文件都会被拒绝,解析后落在工作区之外的路径同样被拒绝;不确定文件是否存在时,先用 search_text 查找。

成功时返回的是一段纯文本,由三部分组成。第一行是摘要,形如:

内容是否截断: false | bytes: 1521 | runes: 1521 | lines: 97

第二行是单独的"正文:",其后是逐行渲染的文件正文。

摘要中四个值的口径。bytes 和 runes 是编号之前那部分正文的字节数和字符数(Unicode 码点数),按原始字节统计,保留 '\r' 和换行符,因此不等于正文渲染后的字节数或字符数;文件未被截断时,它们就是整个文件的量。lines 是整个文件的行数,由一次独立的全文扫描得出,空文件为 0,末尾没有换行符的最后一行同样计入。truncated 表示正文是否被截断。

正文的每一行对应文件中的一行,行首加上从 1 开始的行号、' | ' 和该行文本,行号按本次渲染出的行数的十进制位数右对齐,例如:

  7 | package workspace

行尾的一个 '\r' 会被去掉,每行都以换行结束。行号和分隔符是附加的,不属于文件原内容。

读取的正文字节上限是 1000000。文件更大时,只返回前 1000000 字节的渲染结果,并丢弃被这个上限切断的多字节字符的残缺字节,这些字节不出现在正文里,所以正文的最后一行是原文件对应行的前半段。此时 truncated 为 true,bytes 可能略小于 1000000,不能用它判断文件总大小,而 lines 仍是整个文件的行数,会大于正文实际包含的行数。除了单行过长的情形,未截断时 lines 等于正文行数,截断时大于正文行数。工具没有偏移或长度参数,truncated 为 true 时无法继续读取剩余内容,应改用 search_text 定位需要部分,或只读取确实需要的文件。

如果文件中存在单行超过 10485760 字节的行,行数无法统计,此时会同时返回错误和已读取的正文,而摘要中的 lines 为 0。这是 lines 为 0 而正文非空的唯一情况。

读取失败时不返回文件内容,只返回一段说明失败原因的文本,例如路径被拒绝、文件类型不受支持、不是普通文件或读取过程出错,内容中会指出具体路径。如果失败发生在正文已读入之后,返回的文本会先给出失败原因,再接上已经读取到的部分结果。。`,
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

	executeResult, err := r.workspace.ReadTextFile(args.Path)
	result := readTextFileToolResultResponse(executeResult)

	if err != nil {
		return result, fmt.Errorf("使用 read_text_file 工具获取文件 %q 内容失败, 因为: %w", args.Path, err)
	}

	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("执行 read_text_file 工具后, 即将返回结果时失败, 因为: %w", err)
	}

	return result, nil
}

var _ tool.Tool = (*ReadTextFileTool)(nil)
