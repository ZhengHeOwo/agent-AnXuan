package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
)

// ToolRegistry 保存工具实现，并提供工具定义列表和名称查找能力。
type ToolRegistry struct {
	tools       map[string]Tool
	definitions []model.ToolDefinition
}

// NewToolRegistry 校验并注册工具。
func NewToolRegistry(tools ...Tool) (*ToolRegistry, error) {
	ToolRegistry := &ToolRegistry{
		tools:       make(map[string]Tool, len(tools)),
		definitions: make([]model.ToolDefinition, 0, len(tools)),
	}

	for _, candidate := range tools {
		if candidate == nil {
			return nil, fmt.Errorf("工具不能为空")
		}

		definition := candidate.Definition()

		name := strings.TrimSpace(definition.Name)

		if name == "" {
			return nil, fmt.Errorf("工具名不能为空")
		}

		if _, exists := ToolRegistry.tools[name]; exists {
			return nil, fmt.Errorf("工具名 %q 已存在", name)
		}

		description := strings.TrimSpace(definition.Description)
		if description == "" {
			return nil, fmt.Errorf("工具 %q 的描述不能为空", name)
		}

		if !json.Valid(definition.Parameters) {
			return nil, fmt.Errorf("工具 %q 的参数 Schema 不是合法JSON", name)
		}

		definition.Name = name
		definition.Description = description
		ToolRegistry.tools[name] = candidate
		ToolRegistry.definitions = append(ToolRegistry.definitions, definition)
	}

	return ToolRegistry, nil
}

// Get 按工具名称查找已注册工具。
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}

	registeredTool, exists := r.tools[strings.TrimSpace(name)]
	return registeredTool, exists
}

// Definitions 返回工具定义的副本。
func (r *ToolRegistry) Definitions() []model.ToolDefinition {
	if r == nil {
		return nil
	}

	definitions := make([]model.ToolDefinition, len(r.definitions))
	copy(definitions, r.definitions)

	return definitions
}
