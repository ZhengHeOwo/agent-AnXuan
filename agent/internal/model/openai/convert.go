package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
)

func toModelResponse(response chatCompletionResponse) (model.Response, error) {
	if len(response.Choices) == 0 {
		return model.Response{}, fmt.Errorf("模型响应Choices为空")
	}

	choices := response.Choices[0]

	modelMessage, err := toModelMessage(choices.Message)
	if err != nil {
		return model.Response{}, err
	}

	modelResponse := model.Response{
		Message:      modelMessage,
		FinishReason: choices.FinishReason,
	}

	return modelResponse, nil
}

func toModelMessage(chatMessage chatMessage) (model.Message, error) {
	content := ""
	if chatMessage.Content != nil {
		content = *chatMessage.Content
	}

	var role model.Role
	switch chatMessage.Role {
	case "system":
		role = model.RoleSystem
	case "user":
		role = model.RoleUser
	case "assistant":
		role = model.RoleAssistant
	case "tool":
		role = model.RoleTool
	default:
		return model.Message{}, fmt.Errorf("Role类型不符合预设: %q", chatMessage.Role)
	}

	modelMessage := model.Message{
		Role:             role,
		Content:          content,
		ReasoningContent: chatMessage.ReasoningContent,
		ToolCallID:       chatMessage.ToolCallID,
	}

	if len(chatMessage.ToolCalls) == 0 {
		return modelMessage, nil
	}

	toolCalls := make([]model.ToolCall, 0, len(chatMessage.ToolCalls))

	for _, chatToolCall := range chatMessage.ToolCalls {
		switch chatToolCall.Type {
		case "function":

		default:
			return model.Message{}, fmt.Errorf("错误的工具类型: %q", chatToolCall.Type)
		}

		modelID := chatToolCall.ID
		modelName := chatToolCall.Function.Name
		modelArguments := json.RawMessage(chatToolCall.Function.Arguments)

		toolCalls = append(toolCalls, model.ToolCall{
			ID:        modelID,
			Name:      modelName,
			Arguments: modelArguments,
		})
	}

	modelMessage.ToolCalls = toolCalls

	return modelMessage, nil
}

func toChatCompletionRequest(request model.Request) (chatCompletionRequest, error) {
	if len(request.Messages) == 0 {
		return chatCompletionRequest{}, fmt.Errorf("内部请求信息的Messages为空, 无法转换为外部请求")
	}

	reqMessages := make([]chatMessage, 0, len(request.Messages))
	for _, modelReqMessage := range request.Messages {
		chatMessage, err := toChatMessage(modelReqMessage)
		if err != nil {
			return chatCompletionRequest{}, err
		}
		reqMessages = append(reqMessages, chatMessage)
	}

	var chatTools []chatToolDefinition
	if len(request.Tools) > 0 {
		chatTools = make([]chatToolDefinition, 0, len(request.Tools))

		for _, chatTool := range request.Tools {
			chatDefinition, err := toChatToolDefinition(chatTool)
			if err != nil {
				return chatCompletionRequest{}, err
			}

			chatTools = append(chatTools, chatDefinition)
		}
	}

	chatCompletionRequest := chatCompletionRequest{
		Model:    request.Model,
		Messages: reqMessages,
		Tools:    chatTools,
	}
	return chatCompletionRequest, nil
}

func toChatMessage(modelMessage model.Message) (chatMessage, error) {
	var content *string
	if modelMessage.Content != "" || len(modelMessage.ToolCalls) == 0 {
		content = &modelMessage.Content
	}

	var role string
	switch modelMessage.Role {
	case model.RoleSystem:
		role = "system"
	case model.RoleUser:
		role = "user"
	case model.RoleAssistant:
		role = "assistant"
	case model.RoleTool:
		role = "tool"
	default:
		return chatMessage{}, fmt.Errorf("Role类型不符合预设: %q", modelMessage.Role)
	}

	chatToolCalls := make([]chatToolCall, 0, len(modelMessage.ToolCalls))
	for _, toolCall := range modelMessage.ToolCalls {
		chatToolCall := chatToolCall{
			ID:   toolCall.ID,
			Type: "function",
			Function: chatFunctionCall{
				Name:      toolCall.Name,
				Arguments: string(toolCall.Arguments),
			},
		}

		chatToolCalls = append(chatToolCalls, chatToolCall)
	}
	chatMessage := chatMessage{
		Role:             role,
		Content:          content,
		ReasoningContent: modelMessage.ReasoningContent,
		ToolCalls:        chatToolCalls,
		ToolCallID:       modelMessage.ToolCallID,
	}

	return chatMessage, nil
}

func toChatToolDefinition(definition model.ToolDefinition) (chatToolDefinition, error) {
	name := strings.TrimSpace(definition.Name)
	if name == "" {
		return chatToolDefinition{}, fmt.Errorf("工具名称不能为空")
	}

	description := strings.TrimSpace(definition.Description)
	if description == "" {
		return chatToolDefinition{}, fmt.Errorf("工具 %q 的描述不能为空", name)
	}

	if !json.Valid(definition.Parameters) {
		return chatToolDefinition{}, fmt.Errorf("工具 %q 的参数 Schema 不是合法JSON", name)
	}

	return chatToolDefinition{
		Type: "function",
		Function: chatFunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  definition.Parameters,
		},
	}, nil
}
