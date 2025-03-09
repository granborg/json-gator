package main

import (
	"encoding/json"
	"fmt"

	v8 "rogchap.com/v8go"
)

func ConvertGoToJavaScript(ctx *v8.Context, goObj any) (*v8.Value, error) {
	// Step 1: Serialize the Go object to JSON
	jsonData, err := json.Marshal(goObj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Go object: %w", err)
	}

	// Step 2: Create a JavaScript object by parsing the JSON
	jsValue, err := ctx.RunScript(fmt.Sprintf("JSON.parse(%q)", string(jsonData)), "conversion.js")
	if err != nil {
		return nil, fmt.Errorf("failed to create JS object: %w", err)
	}

	return jsValue, nil
}

func ConvertJavaScriptToGo(ctx *v8.Context, jsValue *v8.Value) (any, error) {
	// Step 1: Convert the JavaScript value to a JSON string
	jsonStringScript := fmt.Sprintf("JSON.stringify(%s)", jsValue.String())
	jsonValue, err := ctx.RunScript(jsonStringScript, "stringify.js")
	if err != nil {
		return nil, fmt.Errorf("failed to stringify JavaScript value: %w", err)
	}

	// Step 2: Unmarshal the JSON string into the target Go object
	var target any
	err = json.Unmarshal([]byte(jsonValue.String()), &target)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JavaScript value: %w", err)
	}

	return target, nil
}
