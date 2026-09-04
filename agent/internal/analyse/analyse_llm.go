package analyse

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

const maxAnalyseModelSteps = 16

type analyseRuntime struct {
	llm       model.Model
	modelName string
	messages  []model.Message
	tools     *tool.Registry
	store     *preferencesStore
}

func NewAnalyseRuntime(
	llm model.Model,
	modelName string,
	tools *tool.Registry,
	databaseKeys string,
	store *preferencesStore,
) (*analyseRuntime, error) {
	if llm == nil {
		return nil, fmt.Errorf("Model must not be empty")
	}

	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("ModelName must not be empty")
	}

	systemPrompt := strings.TrimSpace(os.Getenv("ANALYSE_SYSTEM_PROMPT"))
	if systemPrompt == "" {
		return nil, fmt.Errorf("systemPrompt must not be empty")
	}

	if strings.TrimSpace(databaseKeys) == "" {
		return nil, fmt.Errorf(
			"databaseKeys must not be empty",
		)
	}

	systemPrompt += "\n\n" + databaseKeys

	if tools == nil {
		return nil, fmt.Errorf("tool registry must not be empty")
	}

	analyseRuntime := &analyseRuntime{
		llm:       llm,
		modelName: strings.TrimSpace(modelName),
		messages: []model.Message{
			model.Message{
				Role:    model.RoleSystem,
				Content: systemPrompt,
			},
		},
		tools: tools,
		store: store,
	}

	return analyseRuntime, nil
}

// analyseRunTurn 执行分析模型。
func (r *analyseRuntime) analyseRunTurn(ctx context.Context, input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("input 不能为空")
	}

	workingMessages := make([]model.Message, 0, len(r.messages)+1)
	workingMessages = append(workingMessages, r.messages...)
	workingMessages = append(workingMessages, model.Message{
		Role:    model.RoleUser,
		Content: input,
	})

	for step := 0; step < maxAnalyseModelSteps; step++ {
		response, err := r.llm.Complete(ctx, model.Request{
			Model:    r.modelName,
			Messages: workingMessages,
			Tools:    r.tools.Definitions(),
		})
		if err != nil {
			return fmt.Errorf("Model invocation failed: %w", err)
		}

		message := response.Message
		if message.Role != model.RoleAssistant {
			return fmt.Errorf(
				"Err Model Role: want %q, got: %q",
				model.RoleAssistant,
				message.Role,
			)
		}

		if len(message.ToolCalls) == 0 {
			workingMessages = append(workingMessages, message)
			r.messages = workingMessages
			if strings.TrimSpace(message.Content) == "NO_ACTION" {
				return errNoAction
			}

			return fmt.Errorf(
				"Model abnormal text response: %q",
				message.Content,
			)
		}

		workingMessages = append(workingMessages, message)

		for _, call := range message.ToolCalls {
			if strings.TrimSpace(call.Name) == "" {
				return fmt.Errorf(
					"Analyse Model call Tool Missing Tool Name",
				)
			}

			if strings.TrimSpace(call.ID) == "" {
				return fmt.Errorf(
					"Analyse Model call Tool Missing Tool ID",
				)
			}

			result := r.analyseModelExecuteToolCall(ctx, call)

			workingMessages = append(workingMessages, model.Message{
				Role:       model.RoleTool,
				Content:    result,
				ToolCallID: call.ID,
			})
		}
	}

	return fmt.Errorf(
		"The Analyse model has exceeded the maximum number of steps",
	)
}

func (r *analyseRuntime) analyseModelExecuteToolCall(ctx context.Context, call model.ToolCall) string {
	registeredTool, exists := r.tools.Get(call.Name)
	if !exists {
		return fmt.Sprintf("Analyse Model call Tool Falid, Unregistered tool: %q", call.Name)
	}

	result, err := registeredTool.Execute(ctx, call.Arguments)
	if err != nil {
		return fmt.Sprintf("Tool %q Execution failed: %v", call.Name, err)
	}

	return result
}
