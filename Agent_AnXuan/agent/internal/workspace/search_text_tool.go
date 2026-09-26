package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

type SearchTextTool struct {
	workspace *Workspace
}

func NewSearchTextTool(workspace *Workspace) (*SearchTextTool, error) {
	if workspace == nil || workspace.root == nil {
		return nil, fmt.Errorf("create search_text tool: workspace is nil")
	}

	return &SearchTextTool{
		workspace: workspace,
	}, nil
}

var searchTextParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "A non-empty, case-sensitive literal substring to find. Regular expressions, fuzzy matching, and case-insensitive matching are not supported. Use a specific query when a previous result was truncated."
    }
  },
  "required": ["query"],
  "additionalProperties": false
}`)

func (t *SearchTextTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "search_text",
		Description: "递归搜索受控工作区内的受支持文本文件,查找一段区分大小写的字面量子串。query 必须非空且不能是空白,上限 1024 字节,按字节精确匹配;不支持正则表达式、模糊匹配、大小写忽略,也不能按路径、目录或文件类型筛选。工作区根的位置以及它在磁盘上的绝对位置不会暴露,结果中的路径都是工作区相对路径,使用 '/' 分隔,可能包含多层目录。\n" +
			"搜索范围与 list_text_files 能列出的文件一致,即文件名恰为 go.mod 或 go.sum,或扩展名恰为 .go、.md、.txt、.json、.yaml、.yml、.toml 的普通文件;目录、符号链接和其他非普通文件不参与,路径名本身也不参与匹配,匹配的只是文件内容。\n" +
			"成功时返回的是一段纯文本,由三部分组成。第一行是摘要,形如:\n" +
			"\n" +
			"候选列表是否被截断: false | 搜索内容是否被截断: false | 匹配条数: 1\n" +
			"\n" +
			"第二行是单独的 \"搜索结果:\",其后每一行是一条匹配,形如:\n" +
			"\n" +
			"路径: \"Agent-AnXuan/go.mod\" | 行: 3 | 文本是否被截断: false | 文本: module github.com/ZhengHeOwo/agent-AnXuan\n" +
			"\n" +
			"没有匹配时匹配条数为 0,不再输出 搜索结果 行,也不会有任何匹配行。\n" +
			"摘要中三个值的口径。候选列表是否被截断表示候选文件列表本身不完整,与 list_text_files 的截断同源,即遍历到的条目(含目录和符号链接)超过 2000 个,或已列出的文件达到 1500 个;此时有一部分文件根本没有被搜索过,缺少的是遍历顺序靠后的部分。\n" +
			"搜索内容是否被截断表示本次结果可能不完整,出现下面任意一种情况都为 true:某个文件的命中达到 20 条、累计命中达到 100 条、某个文件超过 8388608 字节(8 MiB)而完全没有被搜索,或某个文件在扫描中途失败(单行达到 262144 字节及以上,或读取出错)。\n" +
			"每条匹配四个字段的口径。路径按 Go 字符串字面量给出,反斜杠和引号会转义;行是从 1 开始的行号;文本是否被截断表示该行内容是否被缩短,为 true 时被切断的一侧带有 \"...\";文本是该行的内容,不做转义,行尾换行符不计入,CRLF 行尾的 \\r 会被去掉,所以文本里出现 \" | \" 也不是字段分隔。\n" +
			"文本通常是整行原文;单行超过 2048 字节时,只返回以匹配位置为中心的一段 2048 字节片段,被切断的一侧补上 \"...\",此时文本是否被截断为 true,该字段最长 2054 字节。\n" +
			"单个文件最多返回 20 条命中,达到之后即使该文件还有匹配也不再返回;整个结果最多 100 条命中,按文件顺序累加,达到 100 条后其余文件的命中全部舍弃。结果按文件路径顺序返回(与 list_text_files 的遍历顺序一致,即目录深度优先、同一目录内按名称排序),同一文件内按行号升序;因此用常见词搜索时,路径靠后的文件更容易整段缺失,应改用更具体的 query。\n" +
			"不返回匹配行的上下文,也不返回文件的总行数;需要上下文时改用 read_text_file。工具没有路径、偏移或分页参数,搜索内容是否被截断为 true 时,只能把 query 收窄,或直接读取具体文件。\n" +
			"搜索过程中的问题不会中断整次搜索,而是附在结果末尾一起返回,这种情况仍算成功。文件出问题的情况包括:文件不符合搜索要求(例如类型不支持、路径非法)、打开失败、不是普通文件、超过 8388608 字节(8 MiB)而完全没有被搜索、单行达到 262144 字节及以上、读取出错;除完全没被搜索的文件外,出问题的文件在失败之前已经找到的命中会照常返回。问题说明给出问题总数、各类数量和问题详情,详情按类别分组、组内按路径排序,每条给出路径,能给出原因时附上原因;除文件过大外,每类最多列出 3 条详情,文件过大则每一条都列出。\n" +
			"query 为空白或超过 1024 字节时直接报错,不返回结果;其他失败会先给出失败原因,再附上已经收集到的结果。",
		Parameters: searchTextParameters,
	}
}

type searchTextArguments struct {
	Query string `json:"query"`
}

func (t *SearchTextTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 search_text 工具前失败, 因为: %w", err)
	}

	args, err := tool.DecodeObjectArguments[searchTextArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"解析 search_text 参数失败, 因为: %w",
			err,
		)
	}

	result, err := t.workspace.SearchText(
		ctx,
		args.Query,
	)
	searchResult := searchTextToolResultResponse(result)

	var collector *issueCollector
	if errors.As(err, &collector) {
		return searchResult + searchIssueToolResultResponse(collector), nil
	}

	if err != nil {
		return searchResult, fmt.Errorf(
			"通过 search_text 工具在工作区搜索文本失败, 因为: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return searchResult, fmt.Errorf("执行 search_text 工具后, 即将返回结果时失败, 因为: %w", err)
	}

	return searchResult, nil
}

var _ tool.Tool = (*SearchTextTool)(nil)
