package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

const testModelSystemPrompt = "test-SystemPrompt" // 没有特殊[提示词]的测试需求时 使用该变量即可
const testUserInput = "Hi"                        // 没有特殊[用户输入]的测试需求时 使用该变量即可
const testAssistantContent = "你好"                 // 没有特殊[模型响应的Content字段]的测试需求时 使用该变量即可

type fakeModel struct {
	LastRequest model.Request // 测试例中不用填这个
	Response    model.Response
	err         error
}

func (f *fakeModel) Complete(ctx context.Context, request model.Request) (model.Response, error) {
	f.LastRequest = request // 将runturn的candidate这个第一轮的提示词+输入的Messages赋值给fakeModel的字段,后续测试直接比对,用来确认模型收到的上下文是正确的
	return f.Response, f.err
}

func newEmptyTestRegistry(t *testing.T) *tool.Registry {
	t.Helper()

	toolsRegistry, err := tool.NewRegistry()
	if err != nil {
		t.Fatalf("创建空测试注册表失败: %v", err)
	}
	return toolsRegistry
}

var _ model.Model = (*fakeModel)(nil) // 编译器确认fakeModel实现了model.Model接口

func TestRunTurn(t *testing.T) {
	tests := []struct {
		name             string
		testFakeModel    *fakeModel
		testModelName    string
		testSystemPrompt string
		testInput        string
		testTools        []model.ToolDefinition
		wantErr          error
		wantMessages     []model.Message
		wantStr          string
	}{
		{
			name: "成功调用",
			testFakeModel: &fakeModel{
				Response: model.Response{
					Message: model.Message{
						Role:    model.RoleAssistant,
						Content: testAssistantContent,
					},
					FinishReason: "stop",
				},
				err: nil,
			},
			testModelName:    "test-model",
			testSystemPrompt: testModelSystemPrompt,
			testInput:        testUserInput,
			testTools:        []model.ToolDefinition{},
			wantErr:          nil,
			wantMessages: []model.Message{
				model.Message{
					Role:    model.RoleSystem,
					Content: testModelSystemPrompt,
				},
				model.Message{
					Role:    model.RoleUser,
					Content: testUserInput,
				},
				model.Message{
					Role:    model.RoleAssistant,
					Content: testAssistantContent,
				},
			},
			wantStr: testAssistantContent,
		},
		{
			name: "失败调用",
			testFakeModel: &fakeModel{
				Response: model.Response{
					Message: model.Message{
						Role:    model.RoleAssistant,
						Content: testAssistantContent,
					},
					FinishReason: "stop",
				},
				err: fmt.Errorf("[测试-Err]: 调用模型时发生了xxx导致失败"),
			},
			testModelName:    "test-model",
			testSystemPrompt: testModelSystemPrompt,
			testInput:        testUserInput,
			testTools:        []model.ToolDefinition{},
			wantErr:          ErrModelInvocationFailed,
			wantMessages: []model.Message{
				model.Message{
					Role:    model.RoleSystem,
					Content: testModelSystemPrompt,
				},
			},
			wantStr: "",
		},
		{
			name: "响应角色不是 assistant",
			testFakeModel: &fakeModel{
				Response: model.Response{
					Message: model.Message{
						Role:    model.RoleTool,
						Content: testAssistantContent,
					},
					FinishReason: "stop",
				},
				err: nil,
			},
			testModelName:    "test-model",
			testSystemPrompt: testModelSystemPrompt,
			testInput:        testUserInput,
			testTools:        []model.ToolDefinition{},
			wantErr:          ErrResponseRoleError,
			wantMessages: []model.Message{
				model.Message{
					Role:    model.RoleSystem,
					Content: testModelSystemPrompt,
				},
			},
			wantStr: "",
		},
		{
			name: "响应 Content 为空[1]",
			testFakeModel: &fakeModel{
				Response: model.Response{
					Message: model.Message{
						Role:    model.RoleAssistant,
						Content: "",
					},
					FinishReason: "stop",
				},
				err: nil,
			},
			testModelName:    "test-model",
			testSystemPrompt: testModelSystemPrompt,
			testInput:        testUserInput,
			testTools:        []model.ToolDefinition{},
			wantErr:          ErrEmptyContent,
			wantMessages: []model.Message{
				model.Message{
					Role:    model.RoleSystem,
					Content: testModelSystemPrompt,
				},
			},
			wantStr: "",
		},
		{
			name: "响应 Content 为空[2]",
			testFakeModel: &fakeModel{
				Response: model.Response{
					Message: model.Message{
						Role:    model.RoleAssistant,
						Content: " \n\t",
					},
					FinishReason: "stop",
				},
				err: nil,
			},
			testModelName:    "test-model",
			testSystemPrompt: testModelSystemPrompt,
			testInput:        testUserInput,
			testTools:        []model.ToolDefinition{},
			wantErr:          ErrEmptyContent,
			wantMessages: []model.Message{
				model.Message{
					Role:    model.RoleSystem,
					Content: testModelSystemPrompt,
				},
			},
			wantStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := NewRuntime(tt.testFakeModel, tt.testModelName, tt.testSystemPrompt, newEmptyTestRegistry(t)) // 拿到Runtime结构体
			if err != nil {
				t.Fatalf("调用[NewRuntime]获取[Runtime]结构体失败, 错误: %v", err)
			}

			if len(runtime.Messages) != 1 {
				t.Fatalf("调用[NewRuntime]获取的[Runtime]结构体的[messages字段]错误, want Len(): %d, got Len(): %d", 1, len(runtime.Messages))
			}

			wantSystemMessage := model.Message{ // 期望Runtime创建时messages只含有一份系统提示词的model.Message
				Role:    model.RoleSystem,
				Content: tt.testSystemPrompt,
			}

			if !reflect.DeepEqual(runtime.Messages[0], wantSystemMessage) {
				t.Fatalf("调用[NewRuntime]获取的[Runtime]结构体的[messages字段]错误, want: %v, got: %v", wantSystemMessage, runtime.Messages[0])
			}

			gotStr, err := runtime.RunTurn(context.Background(), tt.testInput)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("实际错误信息链中不包含期望的哨兵错误, wantErr: %v, gotErr: %v", tt.wantErr, err)
				}

				if gotStr != "" {
					t.Fatalf("出现错误时应该返回空字符串,但出现了值: %s", gotStr)
				}
			}

			if tt.wantErr == nil && err != nil {
				t.Fatalf("期望正确的测试出现错误, err: %v", err)
			}
			if gotStr != tt.wantStr {
				t.Fatalf("期望调用[RunTurn]得到的string与实际不符, want: %s, got: %s", tt.wantStr, gotStr)
			}

			candidate := make([]model.Message, 0, len(runtime.Messages)) // 准备拼接系统提示词和输入 组成固定的第一轮请求信息
			candidate = append(candidate, runtime.Messages[0])
			candidate = append(candidate, model.Message{
				Role:    model.RoleUser,
				Content: tt.testInput,
			})

			wantModelRequest := model.Request{
				Model:    runtime.modelName,
				Messages: candidate,
				Tools:    tt.testTools,
			}

			if !reflect.DeepEqual((*tt.testFakeModel).LastRequest, wantModelRequest) { // 将期望请求和假模型内收到的真实请求对比
				t.Fatalf("期望调用[Complete]时传入的model.Request错误, want: %v, got: %v", wantModelRequest, (*tt.testFakeModel).LastRequest)
			}

			if !reflect.DeepEqual(runtime.Messages, tt.wantMessages) {
				t.Fatalf("调用[Complete]一轮结束后 此时的runtime.messages与期望得到的结构不符, want: %v, got: %v", tt.wantMessages, runtime.Messages)
			}
		})
	}
}

type runtimeFakeTool struct {
	definition model.ToolDefinition
}

func (f *runtimeFakeTool) Definition() model.ToolDefinition {
	return f.definition
}

func (f *runtimeFakeTool) Execute(
	ctx context.Context,
	arguments json.RawMessage,
) (string, error) {
	return "test-result", nil
}

func TestRunTurnIncludesToolDefinitions(t *testing.T) {
	var testTool *runtimeFakeTool = &runtimeFakeTool{
		definition: model.ToolDefinition{
			Name:        "test-tool",
			Description: "test-工具描述",
			Parameters:  json.RawMessage(`{}`),
		},
	}

	testRegistry, err := tool.NewRegistry(testTool)
	if err != nil {
		t.Fatalf("创建注册表失败: %v", err)
	}

	testModel := &fakeModel{
		Response: model.Response{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "测试响应",
			},
			FinishReason: "stop",
		},
		err: nil,
	}
	runtime, err := NewRuntime(testModel, "test-model", "测试提示词", testRegistry)
	if err != nil {
		t.Fatalf("调用[NewRuntime]获取[Runtime]结构体失败, 错误: %v", err)
	}

	_, err = runtime.RunTurn(context.Background(), "测试输入")
	if err != nil {
		t.Fatalf("RunTurn执行失败: %v", err)
	}

	if len(testModel.LastRequest.Tools) != 1 {
		t.Fatalf("want 1, got: %d", len(testModel.LastRequest.Tools))
	}

	name := testModel.LastRequest.Tools[0].Name
	if name != "test-tool" {
		t.Fatalf("want: %s, got: %s", "test-tool", name)
	}
}

type scriptedModel struct {
	requests  []model.Request
	responses []model.Response
	index     int
}

func (m *scriptedModel) Complete(ctx context.Context, request model.Request) (model.Response, error) {
	m.requests = append(m.requests, request)

	if m.index >= len(m.responses) {
		return model.Response{}, fmt.Errorf("没有更多测试响应")
	}
	response := m.responses[m.index]
	m.index++

	return response, nil
}

type fakeTool struct{}

func (f *fakeTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name:        "test_read_file",
		Description: "test-工具描述",
		Parameters: json.RawMessage(`{
	"type": "object",
	"properties": {
		"filename": {"type": "string"}
	},
	"required": ["filename"],
	"additionalProperties": false
}`),
	}
}

func (f *fakeTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if !json.Valid(arguments) {
		return "", fmt.Errorf("测试工具 Schema 参数的JSON不合法")
	}
	if !bytes.Equal(arguments, json.RawMessage(`{"filename":"test_note.txt"}`)) {
		return "", fmt.Errorf("测试工具参数与期望不符,want: %s got: %s", string(json.RawMessage(`{"filename":"test_note.txt"}`)), string(arguments))
	}

	return "test-result", nil
}

var testFileTool = &fakeTool{}

const testUserInput1 string = "查询xx文件"

func TestRunTurn_ToolLoop(t *testing.T) {
	tests := []struct {
		name      string
		testModel *scriptedModel
		testTools []tool.Tool
		wantErr   error
		wantGot   string
	}{
		{
			name: "成功工具循环",
			testModel: &scriptedModel{
				responses: []model.Response{
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role:    model.RoleAssistant,
							Content: "test-result",
						},
					},
				},
			},
			testTools: []tool.Tool{
				testFileTool,
			},
			wantErr: nil,
			wantGot: "test-result",
		},
		{
			name: "超过最大步数",
			testModel: &scriptedModel{
				responses: []model.Response{
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
					model.Response{
						Message: model.Message{
							Role: model.RoleAssistant,
							ToolCalls: []model.ToolCall{
								model.ToolCall{
									ID:        "tool_abc",
									Name:      "test_read_file",
									Arguments: json.RawMessage(`{"filename":"test_note.txt"}`),
								},
							},
						},
					},
				},
			},
			testTools: []tool.Tool{
				testFileTool,
			},
			wantErr: ErrMaxStepsExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRegistry, err := tool.NewRegistry(tt.testTools...)
			if err != nil {
				t.Fatalf("工具注册表创建失败: %v", err)
			}

			testRuntime, err := NewRuntime(tt.testModel, "test-model", "测试-提示词", testRegistry)
			if err != nil {
				t.Fatalf("创建Runtime失败: %v", err)
			}

			got, err := testRuntime.RunTurn(context.Background(), testUserInput1)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("实际错误 %v 中不包含期望错误: %v", err, tt.wantErr)
				}

				if len(testRuntime.Messages) != 1 {
					t.Fatalf("工具循环失败后正式历史应保持不变, want: 1, got: %d", len(testRuntime.Messages))
				}

				return
			}

			if err != nil {
				t.Fatalf("RunTurn出现非预期错误: %v", err)
			}

			if got != tt.wantGot {
				t.Fatalf("模型最终输出与预设不同, 实际:%v", got)
			}

			if tt.testModel.requests[1].Messages[2].Role != model.RoleAssistant {
				t.Fatalf("want: %s, got %s", model.RoleAssistant, tt.testModel.requests[1].Messages[2].Role)
			}

			if len(tt.testModel.requests[1].Messages[2].ToolCalls) != 1 {
				t.Fatalf("want: %d, got %d", 1, len(tt.testModel.requests[1].Messages[2].ToolCalls))
			}

			if tt.testModel.requests[1].Messages[3].Role != model.RoleTool {
				t.Fatalf("want: %s, got %s", model.RoleTool, tt.testModel.requests[1].Messages[3].Role)
			}

			if len(testRuntime.Messages) != 5 {
				t.Fatalf("最终上下文长度错误, want: 5, got: %d", len(testRuntime.Messages))
			}

			if got := tt.testModel.requests[1].Messages[3].ToolCallID; got != "tool_abc" {
				t.Fatalf("ToolCallID错误, want: %q, got: %q", "tool_abc", got)
			}
		})
	}
}
