package tool

import (
	"context"
	"encoding/json"

	"github.com/ZhengHeOwo/agent_an_xuan/agent/internal/model"
)

type Tool interface {
	Definition() model.ToolDefinition
	Execute(ctx context.Context, arguments json.RawMessage) (string, error)
}
