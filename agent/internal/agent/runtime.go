package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

const maxModelSteps = 8

// Runtime 管理对话历史和单轮内的多步工具调用。
type Runtime struct {
	llm       model.Model
	modelName string
	Messages  []model.Message
	tools     *tool.Registry
}

// NewRuntime 创建Agent运行时。
func NewRuntime(llm model.Model, modelName string, systemPrompt string, tools *tool.Registry, preference string) (*Runtime, error) {
	if llm == nil {
		return nil, fmt.Errorf("Model must not be empty")
	}

	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("ModelName must not be empty")
	}

	if strings.TrimSpace(systemPrompt) == "" {
		return nil, fmt.Errorf("systemPrompt must not be empty")
	}

	systemPrompt += preference
	
	if tools == nil {
		return nil, fmt.Errorf("tool registry must not be empty")
	}

	runtime := &Runtime{
		llm:       llm,
		modelName: strings.TrimSpace(modelName),
		Messages: []model.Message{
			model.Message{
				Role:    model.RoleSystem,
				Content: strings.TrimSpace(systemPrompt),
			},
		},
		tools: tools,
	}

	return runtime, nil
}

// RunTurn 执行一轮用户输入。
//
// 只有取得最终模型回答后，本轮候选消息才会提交到正式历史。
func (r *Runtime) RunTurn(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("input 不能为空")
	}

	workingMessages := make([]model.Message, 0, len(r.Messages)+1)
	workingMessages = append(workingMessages, r.Messages...)
	workingMessages = append(workingMessages, model.Message{
		Role:    model.RoleUser,
		Content: input,
	})

	for step := 0; step < maxModelSteps; step++ {
		response, err := r.llm.Complete(ctx, model.Request{
			Model:    r.modelName,
			Messages: workingMessages,
			Tools:    r.tools.Definitions(),
		})
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrModelInvocationFailed, err)
		}

		message := response.Message
		if message.Role != model.RoleAssistant {
			return "", fmt.Errorf("%w: want %q, got: %q", ErrResponseRoleError, model.RoleAssistant, message.Role)
		}

		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return "", ErrEmptyContent
			}

			workingMessages = append(workingMessages, message)
			r.Messages = workingMessages
			return message.Content, nil
		}

		workingMessages = append(workingMessages, message)

		for _, call := range message.ToolCalls {
			if strings.TrimSpace(call.Name) == "" {
				return "", fmt.Errorf("%w: 工具名为空", ErrInvalidToolCall)
			}

			if strings.TrimSpace(call.ID) == "" {
				return "", fmt.Errorf("%w: 工具调用ID为空", ErrInvalidToolCall)
			}

			result := r.executeToolCall(ctx, call)

			workingMessages = append(workingMessages, model.Message{
				Role:       model.RoleTool,
				Content:    result,
				ToolCallID: call.ID,
			})
		}
	}

	return "", ErrMaxStepsExceeded
}

func (r *Runtime) executeToolCall(ctx context.Context, call model.ToolCall) string {
	registeredTool, exists := r.tools.Get(call.Name)
	if !exists {
		return fmt.Sprintf("工具执行失败, 未注册工具: %q", call.Name)
	}

	result, err := registeredTool.Execute(ctx, call.Arguments)
	if err != nil {
		return fmt.Sprintf("工具 %q 执行失败: %v", call.Name, err)
	}

	return result
}
