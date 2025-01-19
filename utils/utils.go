package utils

import (
	"bytes"
	"encoding/json"
)

func InlineIFT[T any](condition bool, a T, b T) T {
	if condition {
		return a
	}
	return b
}

func PrettyJson(data interface{}) (string, error) {
	empty := ""
	tab := "\t"
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent(empty, tab)

	err := encoder.Encode(data)
	if err != nil {
		return empty, err
	}
	return buffer.String(), nil
}
