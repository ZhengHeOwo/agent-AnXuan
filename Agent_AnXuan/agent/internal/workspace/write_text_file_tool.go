package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

// WriteTextFileTool 经狰和确认后，在受控工作区中创建或覆盖文本文件。
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

// maxConfirmationPreviewRunes 限制审批详情里打印的正文长度:
// 更长的正文只展示开头这一段, 落盘内容不受影响。
const maxConfirmationPreviewRunes = 2_000

// confirmationContentPreview 截取正文开头 maxConfirmationPreviewRunes 个码点,
// 第二个返回值表示正文是否被截短。按码点边界切分, 不会切断多字节字符;
// 短于上限的正文被完整遍历一遍, 长于上限的正文只遍历到上限处。
func confirmationContentPreview(content string) (string, bool) {
	characterCount := 0
	for index := range content {
		if characterCount == maxConfirmationPreviewRunes {
			return content[:index], true
		}

		characterCount++
	}

	return content, false
}

var writeTextFileParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "目标文件的工作区相对路径，使用 '/' 分隔，例如 internal/config/config.go。不要填写工作区真实根目录或绝对路径，不要使用反斜杠、冒号、空路径段、'.' 或 '..' 路径段。仅支持文件名恰为 go.mod、go.sum，或扩展名为 .go、.md、.txt、.json、.yaml、.yml、.toml 的文件，区分大小写。获批后会尝试创建缺失的父目录；已有目标必须是普通文件，路径中的符号链接会被拒绝。"
    },
    "content": {
      "type": "string",
      "description": "目标文件最终应保存的完整正文，不是补丁、差异、追加片段或修改指令。已有文件将被整体替换，需要保留的原内容也必须包含在此字符串中。不得为空或仅含空白，同时不得超过 1000000000 字节（十进制 1 GB）和 1000000000 个 Unicode 码点；超限会报错，不会截断。不要包含读取工具附加的摘要、行号、' | ' 分隔符或用于展示代码的围栏，除非它们本来就是需要写入文件的正文。"
    }
  },
  "required": ["path", "content"],
  "additionalProperties": false
}`)

func (t *WriteTextFileTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "write_text_file",
		Description: "经狰和审批后，在受控工作区中创建或完整覆盖一个文本文件。\n" +
			"path 使用工作区相对路径和 '/' 分隔符，支持多层目录，例如 internal/config/config.go；不需要知道工作区真实根目录。绝对路径、反斜杠、冒号、空路径段以及 '.'、'..' 路径段不受支持。\n" +
			"支持文件名恰为 go.mod、go.sum，以及扩展名为 .go、.md、.txt、.json、.yaml、.yml、.toml 的文件，区分大小写。已有目标必须是普通文件，目录和其他非普通文件不能作为写入目标，路径中的符号链接会被拒绝。\n" +
			"content 必须是文件最终的完整正文。该工具不支持追加、局部替换或应用补丁；多次调用同一路径会反复整体覆盖，不能通过分段调用拼接大文件。修改已有文件前，应先读取并确认需要保留的内容；读取结果被截断时，不要把不完整正文当作完整文件覆盖。\n" +
			"读取工具返回的摘要、行号和 ' | ' 分隔符不是原始正文，写回前须移除这些附加内容。不要为了展示代码而给正文额外添加代码围栏。\n" +
			"正文不得为空或仅含空白，且必须同时满足不超过 1000000000 字节（十进制 1 GB）和 1000000000 个 Unicode 码点。Unicode 码点数不是字节数；空白和换行也计入长度。超限直接报错，不会截断。校验通过后按传入正文写入，不自动整理格式、删除首尾空白或补充换行。\n" +
			"调用时先检查参数、路径格式、文件名类型和正文限制，未通过时不会发起审批。通过后，宿主程序向狰和展示目标路径、正文字节数和正文开头（最多 2000 个字符；更长的正文只展示这一段，落盘内容仍是完整正文），并等待狰和明确批准或拒绝。已有文件状态、父目录和符号链接等文件系统检查在获批后的写入阶段进行，因此获批不代表必然写入成功。\n" +
			"狰和明确要求修改且目标与内容清楚时，可直接调用本工具进入审批，无需为了同一次写入再提前询问一次；狰和意图或修改范围不清楚时先澄清。不得自行模拟批准，也不要把调用本工具当作审批已通过。\n" +
			"狰和拒绝时，不执行写入，也不创建父目录。狰和批准后，程序会尝试创建缺失的父目录，再通过同目录临时文件写入并替换目标；不是直接向原文件追加内容。\n" +
			"正常返回的是纯文本，不是 JSON。写入成功时形如：文件 \"internal/config/config.go\" 写入成功；狰和拒绝时形如：写入 \"internal/config/config.go\" 被拒绝(未进行任何更改)。拒绝是一个正常返回结果，不代表写入成功。\n" +
			"发生错误时，根据错误文本判断失败阶段，不要统一宣称未产生任何更改：获批后写入失败可能留下已创建的父目录；若错误明确包含“文件操作已完成”，表示文件写入已完成，只是在返回结果前检测到上下文结束。结果不明确时先读取目标核实，不要盲目重复提交写入。",
		Parameters: writeTextFileParameters,
	}
}

func (t *WriteTextFileTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 write_text_file 工具前中断, 因为: %w", err)
	}

	args, err := tool.DecodeObjectArguments[writeTextFileArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"解析 write_text_file 参数失败, 因为: %w(未产生审批)",
			err,
		)
	}

	toolPath, err := t.workspace.validateTextFileWrite(args.Path, args.Content)
	if err != nil {
		return "", fmt.Errorf(
			"write_text_file 参数未通过执行前校验: %w(未产生审批)",
			err,
		)
	}

	contentPreview, previewTruncated := confirmationContentPreview(args.Content)

	confirmationDetails := fmt.Sprintf(
		"内容长度: %d 字节\n\n内容:\n%s",
		len(args.Content),
		contentPreview,
	)

	if previewTruncated {
		confirmationDetails += fmt.Sprintf(
			"\n\n(正文超过 %d 个字符, 以上只展示开头一段, 落盘内容仍是完整正文)",
			maxConfirmationPreviewRunes,
		)
	}

	confirmed, err := t.confirmer.Confirm(
		ctx,
		tool.ConfirmationRequest{
			Action:  "write_text_file",
			Summary: fmt.Sprintf("创建或覆盖文件 %s", toolPath),
			Details: confirmationDetails,
		},
	)

	if err != nil {
		return "", fmt.Errorf("授权程序中断, 因为: %w", err)
	}

	if !confirmed {
		return writeTextFileToolResultResponse(&writeTextFileResult{
			toolPath: toolPath,
		}), nil
	}

	result, err := t.workspace.WriteTextFile(toolPath, args.Content)
	if err != nil {
		return "", fmt.Errorf(
			"写入 %q 失败, 因为: %w",
			toolPath,
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 write_text_file 工具后, 即将返回结果时失败, 因为: %w(文件操作已完成)", err)
	}

	return writeTextFileToolResultResponse(result), nil
}

var _ tool.Tool = (*WriteTextFileTool)(nil)
