package analyse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

// preferencesGetTool 根据输入'键'，从键值数据库中获取对应值。
type preferencesGetTool struct {
	store *preferencesStore
}

// NewPreferencesGetTool 创建使用键值数据库根据键获取对应值的工具。
func NewPreferencesGetTool(store *preferencesStore) (*preferencesGetTool, error) {
	if store == nil {
		return nil, fmt.Errorf("create preferences_get_tool: store is nil")
	}

	if store.db == nil {
		return nil, fmt.Errorf(
			"create preferences_get_tool: store.db is nil",
		)
	}

	return &preferencesGetTool{
		store: store,
	}, nil
}

type preferencesGetArguments struct {
	Name string `json:"name"`
}

var preferencesGetParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "description": "The exact name of an existing preference key whose stored value should be retrieved."
    }
  },
  "required": ["name"],
  "additionalProperties": false
}`)

func (t *preferencesGetTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name:        "preferences_get_tool",
		Description: "Retrieves the stored value for an existing preference key. Use it to inspect a potentially matching preference before proposing update or rename operations.",
		Parameters:  preferencesGetParameters,
	}
}

func (t *preferencesGetTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	args, err := tool.DecodeObjectArguments[preferencesGetArguments](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse preferences_get_tool arguments: %w",
			err,
		)
	}

	preference, err := t.store.preferenceGet(args.Name)
	if err != nil {
		return "", fmt.Errorf(
			"execute preferences_get_tool: %w",
			err,
		)
	}

	encoded, err := json.Marshal(preference)
	if err != nil {
		return "", fmt.Errorf(
			"marshal preference result: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before result returned: %w", err)
	}

	return string(encoded), nil
}

var _ tool.Tool = (*preferencesGetTool)(nil)

// preferencesOperationSubmitTool 事务提交工具，使分析模型能够提交数据库操作进入事务列表。
type preferencesOperationSubmitTool struct {
	plan *preferencesTransactionPlan
}

// NewPreferencesOperationSubmitTool 创建事务提交工具。
func NewPreferencesOperationSubmitTool(plan *preferencesTransactionPlan) (*preferencesOperationSubmitTool, error) {
	if plan == nil {
		return nil, fmt.Errorf(
			"create preferences_operation_submit_tool: plan is nil",
		)
	}

	return &preferencesOperationSubmitTool{
		plan: plan,
	}, nil
}

// preferencesTransactionPlan 事务提交工具持有该结构体，后续程序通过判断该结构体内信息进行具体数据库操作，
type preferencesTransactionPlan struct {
	Operations []preferencesOperation
}

// preferencesOperation 描述数据库事务内部信息结构。
type preferencesOperation struct {
	operationType preferencesOperationType
	key           string
	newKey        string
	value         string
	reason        string
}

// preferencesOperationDTO -> preferencesOperation 转换函数
func (op *preferencesOperation) UnmarshalJSON(data []byte) error {
	var dto preferencesOperationDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return err
	}

	op.operationType = dto.OperationType
	op.key = dto.Key
	op.newKey = dto.NewKey
	op.value = dto.Value
	op.reason = dto.Reason
	return nil
}

// 数据库操作类型。
type preferencesOperationType string

const (
	preferencesOperationRename preferencesOperationType = "rename"
	preferencesOperationUpdate preferencesOperationType = "update"
	preferencesOperationDelete preferencesOperationType = "delete"
)

type preferencesOperationDTO struct {
	OperationType preferencesOperationType `json:"operation_type"`
	Key           string                   `json:"key"`
	NewKey        string                   `json:"new_key,omitempty"`
	Value         string                   `json:"value,omitempty"`
	Reason        string                   `json:"reason"`
}

var preferencesOperationSubmitToolParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "operation_type": {
      "type": "string",
	  "enum":["rename","update","delete"],
      "description": "The operation to stage: 'rename' renames an existing preference key, 'update' creates or overwrites a preference value, and 'delete' removes an existing preference."
    },
	 "key": {
      "type": "string",
	  "minLength":1,
      "description": "he preference key affected by the operation. For rename and delete, this must be an existing key; for update, it is the key to create or overwrite."
    },
	 "new_key": {
      "type": "string",
	   "minLength":1,
      "description": "The new preference key name. Required only when operation_type is 'rename'."
    },
	 "value": {
      "type": "string",
	   "minLength":1,
      "description": "The preference description to store. Required only when operation_type is 'update'."
    },
	 "reason": {
      "type": "string",
	   "minLength":1,
      "description": "A concise, evidence-based justification for staging the operation, including the preference evidence or conflict being resolved."
    }
  },
  "required": ["operation_type","key","reason"],
    "allOf": [
    {
      "if": {
        "properties": {"operation_type": {"const": "rename"}},
        "required": ["operation_type"]
      },
      "then": {"required": ["new_key"]}
    },
    {
      "if": {
        "properties": {"operation_type": {"const": "update"}},
        "required": ["operation_type"]
      },
      "then": {"required": ["value"]}
    }
  ],
  "additionalProperties": false
}`)

func (t *preferencesOperationSubmitTool) Definition() model.ToolDefinition {
	return model.ToolDefinition{
		Name:        "preferences_operation_submit_tool",
		Description: "Stages one structured preference operation in the pending transaction plan. Use it to propose renaming, updating, or deleting a preference after evaluating the available evidence and inspecting any relevant existing preference. This tool only records the operation; it does not modify the preference store directly.",
		Parameters:  preferencesOperationSubmitToolParameters,
	}
}

func (t *preferencesOperationSubmitTool) Execute(ctx context.Context, arguments json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled before tool execution: %w", err)
	}

	args, err := tool.DecodeObjectArguments[preferencesOperationDTO](arguments)
	if err != nil {
		return "", fmt.Errorf(
			"parse preferences_operation_submit_tool arguments: %w",
			err,
		)
	}

	data, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf(
			"Transaction operation parameter encoding failed: %w",
			err,
		)
	}

	var internalOp preferencesOperation
	if err := json.Unmarshal(data, &internalOp); err != nil {
		return "", fmt.Errorf(
			"Conversion between internal and external structures of transaction parameters failed: %w",
			err,
		)
	}

	if err := transactionParameterVerification(internalOp); err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("The process has already ended before the transaction is committed: %w", err)
	}

	t.plan.Operations = append(t.plan.Operations, internalOp)

	return "Transaction submitted successfully", nil
}

var _ tool.Tool = (*preferencesOperationSubmitTool)(nil)

// 内部结构参数校验函数， 行为未完善， 当前是重复验证。
func transactionParameterVerification(internalOp preferencesOperation) error {
	if internalOp.operationType == "" ||
		strings.TrimSpace(internalOp.key) == "" ||
		strings.TrimSpace(internalOp.reason) == "" {
		return fmt.Errorf(
			"Transaction submission lacks mandatory parameters",
		)
	}

	switch internalOp.operationType {
	case "rename", "update", "delete":

	default:
		return fmt.Errorf(
			"The parameter has used an incorrect operation type: %q",
			internalOp.operationType,
		)
	}

	return nil
}
