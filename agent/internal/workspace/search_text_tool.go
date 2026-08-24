package workspace

import (
	"context"
	"encoding/json"
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
      "description": "Case-sensitive plain text to search for in supported workspace text files"
    }
  },
  "required": ["query"],
  "additionalProperties": false
}`)

func (t *SearchTextTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "search_text",
		Description: "Search supported regular text files in the controlled workspace " +
			"for a case-sensitive plain-text substring. Use this tool to locate relevant " +
			"files and line numbers when the exact file is unknown. Results contain " +
			"workspace-relative paths, 1-based line numbers, and matching line text. " +
			"The result indicates when search limits caused truncation.",
		Parameters: searchTextParameters,
	}
}

type searchTextArguments struct {
	Query string `json:"query"`
}

type searchTextMatchResponse struct {
	Path          string `json:"path"`
	Line          int    `json:"line"`
	Text          string `json:"text"`
	TextTruncated bool   `json:"texttruncated"`
}

type searchTextResponse struct {
	Matches   []searchTextMatchResponse
	Truncated bool
}

func (t *SearchTextTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	args, err := tool.DecodeObjectArguments[searchTextArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse search_text arguments: %w",
			err,
		)
	}

	searchResult, err := t.workspace.SearchText(
		ctx,
		args.Query,
	)

	if err != nil {
		return "", fmt.Errorf(
			"search workspace text: %w",
			err,
		)
	}

	response := searchTextResponse{
		Matches: make(
			[]searchTextMatchResponse,
			0,
			len(searchResult.Matches),
		),
		Truncated: searchResult.Truncated,
	}

	for _, match := range searchResult.Matches {
		response.Matches = append(
			response.Matches,
			searchTextMatchResponse{
				Path:          match.Path,
				Line:          match.Line,
				Text:          match.Text,
				TextTruncated: match.TextTruncated,
			},
		)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf(
			"marshal search_text result: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before result returned: %w", err)
	}

	return string(encoded), nil
}

var _ tool.Tool = (*SearchTextTool)(nil)
