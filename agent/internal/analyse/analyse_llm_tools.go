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
  "description": "暂存一项有证据支持的人格记录操作，供分析正常结束后由程序按提交顺序执行。本次调用不会立即修改数据库，也不会验证目标键是否存在或重命名是否冲突。提交前须按系统提示词完成证据评估、相关记录检索和冲突检查，并考虑此前已暂存的操作；不要假设整个操作序列能够一起回滚。",
  "properties": {
    "operation_type": {
      "type": "string",
      "enum": ["rename", "update", "delete"],
      "description": "本次暂存的操作类型，必须使用原始英文枚举值。rename：修改已有记录的键名及记录中的 Name，保留原 Content 和 Reason；update：新建记录，或完整替换目标记录的 Content 和 Reason，不是追加内容或局部修改；delete：删除已有记录。每次调用仅提交一个操作。"
    },
    "key": {
      "type": "string",
      "minLength": 1,
      "description": "本次操作的目标键。rename 使用旧键，delete 使用待删除键，update 使用待创建或覆盖的键。涉及已有记录时，须先通过 preferences_get_tool 检索其完整内容，不得猜测键名；新建前须检查相关已有记录，避免重复。新键按“人格层级/领域/主题”命名，层级通常为稳定特质、特征适应或叙事身份，使用简体中文并保留必要的技术标识。若操作依赖此前暂存的变更，须确保该键在实际执行到本项时有效，不能把暂存结果当成当前数据库状态。"
    },
    "new_key": {
      "type": "string",
      "minLength": 1,
      "description": "仅 rename 操作必填，其他操作应省略。表示重命名后的完整目标键，遵循与 key 相同的命名规则。实际执行时旧键必须存在，目标键必须不存在；新旧键不能相同。须结合现有键和此前暂存操作检查冲突。重命名不会修改原 Content 和 Reason，需要同时调整内容时另行安排 update 操作。"
    },
    "value": {
      "type": "string",
      "minLength": 1,
      "description": "仅 update 操作必填，其他操作应省略。此字符串将作为记录的完整 Content 保存，并作为 description 提供给主模型，表示助手自身应采用的工作倾向，而不是狰和档案或证据报告。使用简洁、明确、可执行的简体中文，保留领域、适用条件、项目阶段及授权边界；中置信度倾向须采用有条件的表述。更新时整合仍然有效的旧内容，不只提供新增片段。不要写入置信度元数据、原始对话或凭证，也不要把相互矛盾的要求无条件拼接。"
    },
    "reason": {
      "type": "string",
      "minLength": 1,
      "description": "所有操作均必填，说明本次操作的证据与依据。按系统提示词要求，用简体中文依次填写：置信度=<高或中>；人格层级=<稳定特质、特征适应或叙事身份>；证据类型=<明确陈述、行为选择、纠正反馈、结果反馈、自我叙述或其加号组合>；独立事件数=<保守整数>；情境范围=<具体范围或跨情境>；证据摘要=<事实描述>；替代解释=<主要替代解释及结论仍成立的理由>；冲突检查=<与相关已检索记录的比较>；更新依据=<本次操作依据>。中置信度须说明证据限制，不得伪造事件、检索或比较。update 会将此字段保存为新的 Reason；当前程序执行 rename 和 delete 时不保存本次 reason，它仅随操作暂存于内存。Reason 不会提供给主模型，影响人格使用的必要条件必须同时写入 value。"
    }
  },
  "required": ["operation_type", "key", "reason"],
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
