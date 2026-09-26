package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func DecodeObjectArguments[T any](
	arguments json.RawMessage,
) (T, error) {
	var result T

	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return result, fmt.Errorf(
			"tool arguments must be a JSON object",
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf(
			"decode tool arguments: %w",
			err,
		)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return result, fmt.Errorf(
				"tool arguments contain multiple JSON values",
			)
		}

		return result, fmt.Errorf(
			"decode trailing arguments: %w",
			err,
		)
	}

	return result, nil
}
