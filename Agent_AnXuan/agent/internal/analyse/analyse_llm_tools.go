package analyse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ZhengHeOwo/agent_an_xuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent_an_xuan/agent/internal/tool"
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
  "description": "暂存一项有证据支持的人格记录操作，每次调用只提交一项。\n分析对象是狰和，记录的使用主体是主模型。记录应提炼可供主模型内化的关注方式、价值排序、动机取向和判断模式，而不是保存狰和档案、具体任务指令或通用助手最佳实践。仅在证据达到系统提示词规定的入库或修改标准时提交；没有必要操作时不要调用。\n涉及已有记录的更新、重命名或删除，须先通过 preferences_get_tool 读取完整记录；新建前检查可能相关、重复或冲突的已有记录。读取失败不等于记录不存在，已有记录的存在也不证明其归因正确。\n本工具只将操作加入待执行列表，不立即修改数据库，不查询目标键状态，也不验证人格证据和描述是否成立。成功时返回 Transaction submitted successfully，仅表示操作已暂存，不表示数据库已提交。\n后续程序按暂存顺序执行各项操作，整批操作不具备统一的原子回滚保证。提交前须核对操作依赖、键占用和预期结果；查询通常仍反映落库前的状态，不要把暂存的新建或重命名结果当作当前可查询记录，也不要重复提交已成功暂存的操作。",
  "properties": {
    "operation_type": {
      "type": "string",
      "enum": ["rename", "update", "delete"],
      "description": "操作类型，必须使用原始英文枚举值。\nupdate：新建记录，或完整替换目标记录的 Content 和 Reason；不是追加或局部修改。仅补充重要证据时，也须提供完整 value 和 reason，保留仍有效的内容与必要旧证据。\nrename：只修改已有记录的键名及记录中的 Name，保留原 Content 和 Reason。适用于内容仍成立、仅层级或命名范围需要修正的情况；不能仅靠改名把行为规则升级为人格。\ndelete：删除已有记录。仅在明确撤销、充分反证、确认错误归因、实质误导或已被有效整合完全替代时使用。当前对话未提及某项人格不是删除依据，更换分析提示词也不是批量删除依据。"
    },
    "key": {
      "type": "string",
      "minLength": 1,
      "description": "本次操作的完整目标键。update 使用待新建或覆盖的键；rename 使用旧键；delete 使用待删除键。\n新键通常采用“人格层级/心理领域/主题”，第一段为稳定特质、特征适应或叙事身份。用简体中文表达心理含义，保留必要的准确技术标识及领域边界；不要将具体操作清单仅换成心理学名称，也不要仅凭措辞相似合并不同动机或取舍。\n操作现有记录时使用已检索记录的精确键名，不在 key 中顺便修正旧键。若本项依赖此前暂存的重命名，应使用按执行顺序届时有效的键，并以已读取的原记录核验内容，不查询尚未落库的新键来假装完成核验。"
    },
    "new_key": {
      "type": "string",
      "minLength": 1,
      "description": "仅 rename 必填，其他操作省略。填写重命名后的完整目标键，遵循 key 的新键命名规则。\n实际执行时旧键必须存在，目标键必须不存在，因此不能与旧键相同。提交前检查已有键及先前暂存操作对键状态的影响。本字段只改变键名及 Name，不改变 Content 或 Reason；若内容也需要修改，须另行安排 update，并确保操作顺序可执行。"
    },
    "value": {
      "type": "string",
      "minLength": 1,
      "description": "仅 update 必填，其他操作省略。填写记录完整的新 Content，不是差异片段，也不是包含 Name、Content、Reason 的 JSON 对象。此字符串会作为 description 提供给主模型，主模型不接收 reason。\n内容必须源于可归属于狰和的证据，提炼为主模型可以直接采用的内在倾向。描述关注什么、看重什么、如何排序或取舍，并保留会改变含义的领域、阶段、心理条件和边界。可省略主语，使用“重视”“更看重”“倾向”等自然表达；涉及其他人物时保持主体和对象准确。\n不要写成“狰和喜欢”“为了满足狰和”等档案或服务式表述，也不要写成“助手必须”“每次先调用”等步骤命令。具体行为只有在证据支持其背后的倾向时才可抽象；不能从一次要求补造动机、扩大为跨领域特质，或添加通用助手美德和未经支持的代价与例外。\n可信度、证据数量、证据摘要和推断限制写入 reason，不写入 value；必要适用条件仍须保留在 value。“通常”“倾向”可以描述非绝对规律，不代表可信度等级。如果描述只有附加证据不足的免责声明才不致误导，应收窄或暂不提交，不能把免责声明藏入 reason 后保存宽泛结论。\n不保存虚构经历、人格测评分数、诊断、隐秘动机、凭证或无关私密细节；项目方向不得写成已实现能力或自动执行授权。覆盖已有记录时，保留仍成立的心理含义与条件，不因润色或新增一句话而丢失原有有效内容。"
    },
    "reason": {
      "type": "string",
      "minLength": 1,
      "description": "所有操作均必填，填写紧凑、可复核的中文依据，不保存冗长内部逐步思维过程。\n严格按以下字段顺序填写：可信度=<高或中>；人格层级=<稳定特质、特征适应或叙事身份>；构念与命题=<提炼的心理含义>；证据类型=<直接自述、回顾自述、行为选择、纠正反馈、结果反馈或其组合>；证据定位=<可复核的消息位置或已有记录键，仅有本次消息序号时明确注明>；独立事件数=<可确认整数，无法确认时写无法确认>；情境与时间范围=<实际覆盖范围，未知部分注明未知>；支持证据=<必要的事实摘要>；反向证据=<已发现的反例，未观察到时写未观察到，不写不存在>；竞争解释=<主要替代解释及处理结果>；推断限制=<来源、取样、时间或抽象限制>；内化对应=<保留了哪些价值、关注或取舍，未额外增加什么含义>；旧记录比较=<与已读取相关记录的关系，无相关键时如实说明>；操作依据=<新建、证据补充、范围修正、明确撤回、错误归因纠正或整合等>。\n可信度表示证据支持程度，不表示人格强弱。中可信度须写明限制；低可信度不提交。rename 和 delete 的可信度针对本次操作，不能机械沿用旧命题的可信度；不涉及新的内化转写时，在内化对应中如实说明。\n只使用当前材料及已检索记录中明确保存的依据。助手输出、旧记录复述和同一事件的连续澄清不能反复计数；无法确认历史证据是否重叠时不要累加。不得编造日期、事件、检索结果或原始对话，旧记录只有迁移摘要时须保留这一限制。\nupdate 会将本字符串作为完整的新 Reason 保存，应保留仍必要的支持证据、反证、来源与限制，不仅填写本次新增部分。rename 和 delete 的 reason 仅随待执行操作暂存，当前执行逻辑不会将它另行保存为数据库证据或审计记录；rename 后原记录的 Reason 保持不变。"
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
