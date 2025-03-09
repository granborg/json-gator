package main

import (
	"fmt"
	"log"
	"strings"

	v8 "rogchap.com/v8go"
)

type DataModel struct {
	Model           map[string]any      `json:"model"`           // Contains the current values for all topics
	Transformations map[string]any      `json:"transformations"` // key: topic, value: transformation
	Nodes           map[string][]string `json:"nodes"`           // key: datum ID, value: all associated topics
}

func (d *DataModel) applyTransformation(pathTokens []string, data any) (any, error) {
	path := strings.Join(pathTokens, "/")
	transformationAny, exists := d.Transformations[path]
	if !exists {
		return data, fmt.Errorf("did not find a transformation with the path \"%s\"", path)
	}

	// Cast the transformation to the new format
	transformation, ok := transformationAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid transformation format for path '%s': must be an object", path)
	}

	// Extract implementation
	implementation, ok := transformation["implementation"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid transformation format: missing or invalid 'implementation' field")
	}

	// Extract parameters
	parameters := make(map[string]string)
	if params, ok := transformation["parameters"].(map[string]any); ok {
		for k, v := range params {
			if strVal, ok := v.(string); ok {
				parameters[k] = strVal
			} else {
				return nil, fmt.Errorf("parameter '%s' must be a string", k)
			}
		}
	}

	// Create the V8 environment
	iso := v8.NewIsolate()
	defer iso.Dispose()

	// Create the global object template
	global := v8.NewObjectTemplate(iso)

	// Create the context with the global template
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	// Convert Go data to JavaScript object
	jsInput, err := ConvertGoToJavaScript(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data to JavaScript object: %s", err.Error())
	}

	// Set the "self" variable in the global context
	err = ctx.Global().Set("self", jsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to set 'self' in global context: %s", err.Error())
	}

	// Set any parameters in the global context
	for paramName, paramPath := range parameters {
		// Split the path into tokens
		paramPathTokens := strings.Split(paramPath, "/")

		// Recursively resolve the parameter value
		paramValue, err := d.GetModelData(paramPathTokens)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter '%s' at path '%s': %s",
				paramName, paramPath, err.Error())
		}

		// Convert parameter value to JavaScript
		jsParamValue, err := ConvertGoToJavaScript(ctx, paramValue)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter '%s' to JavaScript: %s",
				paramName, err.Error())
		}

		// Set the parameter in the global context
		err = ctx.Global().Set(paramName, jsParamValue)
		if err != nil {
			return nil, fmt.Errorf("failed to set parameter '%s' in global context: %s",
				paramName, err.Error())
		}
	}

	// Run the transformation script
	jsResult, err := ctx.RunScript(implementation, "transformation.js")
	if err != nil {
		return nil, fmt.Errorf("failed to execute JavaScript: %s", err.Error())
	}

	result, err := ConvertJavaScriptToGo(ctx, jsResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JavaScript result \"%s\" to Go object: %s", jsResult, err.Error())
	}
	return result, nil
}

func (d *DataModel) GetModelData(pathTokens []string) (any, error) {
	// Start with the entire model
	var result any = d.Model

	// Navigate through the path to find the requested sub-object
	for _, token := range pathTokens {
		// Check if we're still dealing with a map
		currentMap, ok := result.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path element '%s' does not point to an object", token)
		}

		// Try to get the next element
		nextElement, exists := currentMap[token]
		if !exists {
			return nil, fmt.Errorf("path element '%s' not found", token)
		}

		// Update result to the next element
		result = nextElement
	}

	return result, nil
}

func (d *DataModel) UpdateModelData(pathTokens []string, jsonData any) error {
	// Start with the model map
	subJson := d.Model

	// If there are no path tokens, replace the entire model
	if len(pathTokens) == 0 {
		// Check if jsonData is a map before assigning
		modelMap, ok := jsonData.(map[string]any)
		if !ok {
			return fmt.Errorf("expected JSON object for root model update, got %T", jsonData)
		}

		d.Model = modelMap
		return nil
	}

	// Special case: if we only have one path token
	if len(pathTokens) == 1 {
		subJson[pathTokens[0]] = jsonData
		return nil
	}

	// For deeper paths, we need to ensure the entire path exists
	// Create each level as needed
	current := subJson
	for i := 0; i < len(pathTokens)-1; i++ {
		token := pathTokens[i]

		// Check if this path component exists
		next, exists := current[token]
		if !exists {
			// Path doesn't exist, create a new map
			current[token] = make(map[string]any)
			current = current[token].(map[string]any)
		} else {
			// Path exists, check if it's a map
			nextMap, ok := next.(map[string]any)
			if !ok {
				// It's not a map, replace it with one
				current[token] = make(map[string]any)
				current = current[token].(map[string]any)
			} else {
				current = nextMap
			}
		}
	}

	// Set the final value
	lastToken := pathTokens[len(pathTokens)-1]

	result, err := d.applyTransformation(pathTokens, jsonData)
	if err != nil {
		log.Printf("Transformation failed with error: %s", err.Error())
	}

	current[lastToken] = result
	return nil
}
