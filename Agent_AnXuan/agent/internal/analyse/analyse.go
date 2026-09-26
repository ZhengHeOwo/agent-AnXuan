package analyse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
)

func NewAnalyzeProgramConfiguration(
	llm model.Model,
	modelName string,
	store *preferencesStore,
) (
	*analyseRuntime,
	*preferencesTransactionPlan,
	error,
) {
	var preferencesTransactionPlan = &preferencesTransactionPlan{
		Operations: make([]preferencesOperation, 0),
	}

	preferencesGetTool, err := NewPreferencesGetTool(store)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create preferences_get_tool failed: %w",
			err,
		)
	}

	preferencesOperationSubmitTool, err := NewPreferencesOperationSubmitTool(preferencesTransactionPlan)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create preferences_get_tool failed: %w",
			err,
		)
	}

	tools, err := tool.NewToolRegistry(
		preferencesGetTool,
		preferencesOperationSubmitTool,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Tool ToolRegistry creation failed: %w",
			err,
		)
	}

	databaseKeys, err := store.listPreferenceKeys()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Get Preference Keys list failed: %w",
			err,
		)
	}

	analyseRuntime, err := NewAnalyseRuntime(llm, modelName, tools, databaseKeys, store)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create analyseRuntime failed: %w",
			err,
		)
	}

	return analyseRuntime,
		preferencesTransactionPlan,
		nil
}

// DisplayErrorMessage 收集错误, 用于失败后展示的结构信息.
type DisplayErrorMessage struct {
	ErrorPosition string
	ErrorDetails  string
}

// AnalysisSampleStats 收集分析样本本身的信息.
type AnalysisSampleStats struct {
	Bytes int
	Turns int
}

// UnanalyzedSampleError 整合信息结构体.
type UnanalyzedSampleError struct {
	DisplayErrorMessage DisplayErrorMessage
	AnalysisSampleStats AnalysisSampleStats
}

// AnalyseProcedure 分析程序.
func AnalyseProcedure(
	analyseRuntime *analyseRuntime,
	preferencesTransactionPlan *preferencesTransactionPlan,
	inputMessage []model.Message,
) *UnanalyzedSampleError {
	if analyseRuntime == nil {
		return &UnanalyzedSampleError{
			DisplayErrorMessage: DisplayErrorMessage{
				ErrorPosition: "分析程序入口校验",
				ErrorDetails:  "analyseRuntime 为空",
			},
		}
	}

	if len(inputMessage) < 3 {
		return &UnanalyzedSampleError{
			DisplayErrorMessage: DisplayErrorMessage{
				ErrorPosition: "分析样本数量判断是否小于3",
				ErrorDetails:  "分析样本不符合要求",
			},
		}

	}

	input := inputMessage[1:]
	data, err := json.Marshal(input)
	if err != nil {
		return &UnanalyzedSampleError{
			DisplayErrorMessage: DisplayErrorMessage{
				ErrorPosition: "去除提示词后,为正文进行json编码",
				ErrorDetails:  err.Error(),
			},
		}
	}

	truns := len(input)
	bytes := len(data)

	if err := analyseRuntime.analyseRunTurn(context.Background(), string(data)); err != nil {
		if !errors.Is(err, errNoAction) {
			return &UnanalyzedSampleError{
				DisplayErrorMessage: DisplayErrorMessage{
					ErrorPosition: "调用分析模型",
					ErrorDetails:  err.Error(),
				},
				AnalysisSampleStats: AnalysisSampleStats{
					Bytes: bytes,
					Turns: truns,
				},
			}
		}
	}

	if len(preferencesTransactionPlan.Operations) > 0 {
		for _, operation := range preferencesTransactionPlan.Operations {
			switch operation.operationType {
			case preferencesOperationRename:
				if err := analyseRuntime.store.RenamePreference(operation.key, operation.newKey); err != nil {
					return &UnanalyzedSampleError{
						DisplayErrorMessage: DisplayErrorMessage{
							ErrorPosition: "数据库操作, 执行 RenamePreference",
							ErrorDetails:  err.Error(),
						},
						AnalysisSampleStats: AnalysisSampleStats{
							Bytes: bytes,
							Turns: truns,
						},
					}

				}

			case preferencesOperationUpdate:
				if err := analyseRuntime.store.PutPreference(
					Preference{
						Name:    operation.key,
						Content: operation.value,
						Reason:  operation.reason,
					},
				); err != nil {
					return &UnanalyzedSampleError{
						DisplayErrorMessage: DisplayErrorMessage{
							ErrorPosition: "数据库操作, 执行 PutPreference",
							ErrorDetails:  err.Error(),
						},
						AnalysisSampleStats: AnalysisSampleStats{
							Bytes: bytes,
							Turns: truns,
						},
					}
				}

			case preferencesOperationDelete:
				if err := analyseRuntime.store.DeletePreference(operation.key); err != nil {
					return &UnanalyzedSampleError{
						DisplayErrorMessage: DisplayErrorMessage{
							ErrorPosition: "数据库操作, 执行 DeletePreference",
							ErrorDetails:  err.Error(),
						},
						AnalysisSampleStats: AnalysisSampleStats{
							Bytes: bytes,
							Turns: truns,
						},
					}
				}

			default:
				value := operation.value
				if len(value) > 20 {
					value = string([]rune(value)[:20])
				}
				return &UnanalyzedSampleError{
					DisplayErrorMessage: DisplayErrorMessage{
						ErrorPosition: "数据库操作未注册, 被 default",
						ErrorDetails: fmt.Sprintf("操作类型: %v | key: %v & newKey: %v | value: %v | reason: %v",
							operation.operationType,
							operation.key,
							operation.newKey,
							value,
							operation.reason,
						),
					},
					AnalysisSampleStats: AnalysisSampleStats{
						Bytes: bytes,
						Turns: truns,
					},
				}
			}
		}
	}

	return nil
}

// RetryAnalysis 分析重试.
func RetryAnalysis(analyseRuntime *analyseRuntime, preferencesTransactionPlan *preferencesTransactionPlan, console tool.Confirmer) {
	messages, err := analyseRuntime.store.GotErrBucketValue()
	if err != nil {
		log.Printf("分析重试程序失败, 原因: %v\n程序继续 | 建议立即检查问题并再次重试!\n", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	unanalyzedSampleError := AnalyseProcedure(
		analyseRuntime,
		preferencesTransactionPlan,
		messages,
	)

	if unanalyzedSampleError != nil {
		log.Println("分析重试程序失败, 触发问询操作")

		summary := "未获取到样本详情"
		if unanalyzedSampleError.AnalysisSampleStats.Bytes != 0 || unanalyzedSampleError.AnalysisSampleStats.Turns != 0 {
			summary = fmt.Sprintf("当前样本\nBytes: %d | Turns: %d\n", unanalyzedSampleError.AnalysisSampleStats.Bytes, unanalyzedSampleError.AnalysisSampleStats.Turns)
		}
		confirmed, err := console.Confirm(
			context.Background(),
			tool.ConfirmationRequest{
				Action:  "是否删除未重试成功的未分析样本",
				Summary: summary,
				Details: fmt.Sprintf("错误来源:%q\n原始报错:%q\n", unanalyzedSampleError.DisplayErrorMessage.ErrorPosition, unanalyzedSampleError.DisplayErrorMessage.ErrorDetails),
			},
		)
		if err != nil {
			log.Println("询问失败, 未删除样本, 请及时处理!")
		}

		if !confirmed {
			return
		}
	}

	if err := analyseRuntime.store.DeleteUnprocessedContent(); err != nil {
		log.Printf("删除分析完成的样本失败, 原因: %v\n程序继续 | 样本将在下次收集被覆盖!\n", err)
		return
	}

	log.Println("样本删除成功")
}
