package analyse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
			"create preferences_get_tool faild: %w",
			err,
		)
	}

	preferencesOperationSubmitTool, err := NewPreferencesOperationSubmitTool(preferencesTransactionPlan)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create preferences_get_tool faild: %w",
			err,
		)
	}

	tools, err := tool.NewRegistry(
		preferencesGetTool,
		preferencesOperationSubmitTool,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Tool registry creation failed: %w",
			err,
		)
	}

	databaseKeys, err := store.listPreferenceKeys()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"Get Preference Keys list faild: %w",
			err,
		)
	}

	analyseRuntime, err := NewAnalyseRuntime(llm, modelName, tools, databaseKeys, store)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create analyseRuntime faild: %w",
			err,
		)
	}

	return analyseRuntime,
		preferencesTransactionPlan,
		nil
}

func AnalyseProcedure(
	analyseRuntime *analyseRuntime,
	preferencesTransactionPlan *preferencesTransactionPlan,
	inputMessage []model.Message,
) error {
	if analyseRuntime == nil {
		return fmt.Errorf(
			"analyseRuntime is nil",
		)
	}

	if len(inputMessage) < 3 {
		return fmt.Errorf(
			"The analysis sample does not meet the requirements.",
		)
	}

	input := inputMessage[1:]
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf(
			"Serialization context failed: %w",
			err,
		)
	}

	if err := analyseRuntime.analyseRunTurn(context.Background(), string(data)); err != nil {
		if !errors.Is(err, errNoAction) {
			return err
		}
	}

	if len(preferencesTransactionPlan.Operations) != 0 {
		for _, operation := range preferencesTransactionPlan.Operations {
			switch operation.operationType {
			case preferencesOperationRename:
				if err := analyseRuntime.store.RenamePreference(operation.key, operation.newKey); err != nil {
					return err
				}

			case preferencesOperationUpdate:
				if err := analyseRuntime.store.PutPreference(
					Preference{
						Name:    operation.key,
						Content: operation.value,
						Reason:  operation.reason,
					},
				); err != nil {
					return err
				}

			case preferencesOperationDelete:
				if err := analyseRuntime.store.DeletePreference(operation.key); err != nil {
					return err
				}

			default:
				return fmt.Errorf(
					"Database operation type error",
				)
			}
		}
	}

	return nil
}
