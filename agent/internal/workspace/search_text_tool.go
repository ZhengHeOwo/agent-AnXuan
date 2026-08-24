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
      "description": "A non-empty, case-sensitive literal substring to find. Regular expressions, fuzzy matching, and case-insensitive matching are not supported. Use a specific query when a previous result was truncated."
    }
  },
  "required": ["query"],
  "additionalProperties": false
}`)

func (t *SearchTextTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name: "search_text",
		Description: "Recursively search supported regular text files throughout the controlled " +
			"workspace for a case-sensitive literal substring. This is plain-text search, not regular-" +
			"expression or fuzzy search. The hidden workspace root is never returned; all result paths " +
			"are workspace-relative, use '/' separators, and may include nested directories. Each match " +
			"contains a 1-based line number and either the complete matching line or a bounded snippet " +
			"around the match. text_truncated indicates that an individual line was shortened. This tool " +
			"does not return surrounding lines or total file line counts; call read_text_file when more " +
			"context is required. Results are returned in stable file-path and line order. If the top-" +
			"level truncated field is true, the overall result is incomplete because a file, listing, " +
			"or match limit was reached. Pagination is not currently supported, so use a more specific " +
			"query to narrow an incomplete result.",
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
	TextTruncated bool   `json:"text_truncated"`
}

type searchTextResponse struct {
	Matches   []searchTextMatchResponse `json:"matches"`
	Truncated bool                      `json:"truncated"`
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
