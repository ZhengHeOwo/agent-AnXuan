package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent_an_xuan/Agent_AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent_an_xuan/Agent_AnXuan/agent/internal/tool"
)

// Tool 把 Brave 搜索封装成 web_search 工具。
type Tool struct {
	client *braveClient
}

// NewTool 创建使用 Brave 搜索接口的 web_search 工具。
func NewTool(apiKey string) (*Tool, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("create web_search tool: apiKey is empty")
	}

	return &Tool{
		client: newBraveClient(strings.TrimSpace(apiKey)),
	}, nil
}

type webSearchArguments struct {
	Query string `json:"query"`
}

var webSearchParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "The search query, written like a search engine query. Must not be empty; maximum of 600 characters and 75 words."
    }
  },
  "required": ["query"],
  "additionalProperties": false
}`)

func (t *Tool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "web_search",
		Description: "通过 Brave 搜索接口搜索网页,返回最多 8 条结果,每条包含标题、链接、日期和摘要(摘要外可能再附几段额外摘录;日期和额外摘录不是每条都有)。用于查询需要当前事实或最新信息的问题;返回的是页面片段而不是全文,片段不够用时换更具体的关键词再搜。查询为空会直接报错;0 条结果与请求失败会在返回文本中区分。\n" +
			"参数只有一个 query,按搜索引擎的用法给关键词,上限 600 字符、75 词;引擎会自动修正拼写,若实际使用的查询被修正,返回文本里会标出。",
		Parameters: webSearchParameters,
	}
}

func (t *Tool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("执行 web_search 工具前失败, 因为: %w", err)
	}

	args, err := tool.DecodeObjectArguments[webSearchArguments](arguments)
	if err != nil {
		return "", fmt.Errorf("解析 web_search 参数失败, 因为: %w", err)
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", fmt.Errorf("搜索关键词不能为空")
	}

	response, err := t.client.search(ctx, query)
	if err != nil {
		return "", fmt.Errorf("使用 web_search 工具搜索 %q 失败, 因为: %w", query, err)
	}

	result := webSearchToolResultResponse(query, response)

	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("执行 web_search 工具后, 即将返回结果时失败, 因为: %w", err)
	}

	return result, nil
}

// webSearchToolResultResponse 把搜索结果拼成给模型读的纯文本。
func webSearchToolResultResponse(query string, response *braveSearchResponse) string {
	var builder strings.Builder

	var results []braveWebResult
	if response.Web != nil {
		results = response.Web.Results
	}

	fmt.Fprintf(&builder, "搜索 %q 返回 %d 条", query, len(results))

	altered := strings.TrimSpace(response.Query.Altered)
	original := strings.TrimSpace(response.Query.Original)
	if altered != "" && altered != original {
		fmt.Fprintf(&builder, "\n注: 引擎将查询修正为 %q", altered)
	}

	for index, result := range results {
		fmt.Fprintf(&builder, "\n\n%d. %s", index+1, strings.TrimSpace(result.Title))
		fmt.Fprintf(&builder, "\n   链接: %s", result.URL)

		if date := webSearchResultDate(result); date != "" {
			fmt.Fprintf(&builder, "\n   日期: %s", date)
		}

		if description := strings.TrimSpace(result.Description); description != "" {
			fmt.Fprintf(&builder, "\n   摘要: %s", description)
		}

		for _, snippet := range webSearchExtraSnippets(result) {
			fmt.Fprintf(&builder, "\n   额外摘录: %s", snippet)
		}
	}

	return builder.String()
}

func webSearchResultDate(result braveWebResult) string {
	if date := strings.TrimSpace(result.PageAge); date != "" {
		return date
	}

	return strings.TrimSpace(result.Age)
}

func webSearchExtraSnippets(result braveWebResult) []string {
	seen := make(map[string]bool)

	if description := strings.TrimSpace(result.Description); description != "" {
		seen[description] = true
	}

	snippets := make([]string, 0, len(result.ExtraSnippets))
	for _, snippet := range result.ExtraSnippets {
		snippet = strings.TrimSpace(snippet)
		if snippet == "" || seen[snippet] {
			continue
		}

		seen[snippet] = true
		snippets = append(snippets, snippet)
	}

	return snippets
}

var _ tool.Tool = (*Tool)(nil)
