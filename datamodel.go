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

	// Cache to prevent infinite recursion and improve performance
	transformationCache map[string]any
	processingPaths     map[string]bool
}

// NewDataModel creates a new DataModel with initialized fields
func NewDataModel() *DataModel {
	return &DataModel{
		Model:               make(map[string]any),
		Transformations:     make(map[string]any),
		Nodes:               make(map[string][]string),
		transformationCache: make(map[string]any),
		processingPaths:     make(map[string]bool),
	}
}

// ClearCache clears the transformation cache for a specific path if provided,
// or the entire cache if no path is provided
func (d *DataModel) ClearCache(paths ...string) {
	if len(paths) == 0 {
		// Clear entire cache if no paths are provided
		for k := range d.transformationCache {
			delete(d.transformationCache, k)
		}
	} else {
		// Clear only the specified paths
		for _, path := range paths {
			delete(d.transformationCache, path)
		}
	}
}

// applyTransformation applies a transformation to the given path and returns the result
func (d *DataModel) applyTransformation(path string) (any, error) {
	// Check if already in cache
	if cachedValue, ok := d.transformationCache[path]; ok {
		return cachedValue, nil
	}

	// Check if we're already processing this path (to prevent infinite recursion)
	if d.processingPaths[path] {
		return nil, fmt.Errorf("potential circular dependency detected while processing '%s'", path)
	}

	// Mark this path as being processed
	d.processingPaths[path] = true
	defer func() { delete(d.processingPaths, path) }()

	// Split path into tokens
	pathTokens := strings.Split(path, "/")

	transformationAny, exists := d.Transformations[path]
	if !exists {
		// If no transformation exists, just return the raw value from the model
		return GetMapData(&d.Model, pathTokens)
	}

	// Cast the transformation to the expected format
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

	// Get the current raw value from the model as "self"
	selfValue, err := GetMapData(&d.Model, pathTokens)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		// Only return an error if it's not a "not found" error
		return nil, fmt.Errorf("failed to get raw model data for 'self': %s", err.Error())
	}

	// Convert Go data to JavaScript object
	jsInput, err := ConvertGoToJavaScript(ctx, selfValue)
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
		// Get the parameter value (which might involve recursively applying transformations)
		paramValue, err := d.GetModelData(strings.Split(paramPath, "/"), false)
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
		return nil, fmt.Errorf("failed to convert JavaScript result to Go object: %s", err.Error())
	}

	// Cache the result
	d.transformationCache[path] = result

	return result, nil
}

// applyNestedTransformations applies transformations to all children of a path
func (d *DataModel) applyNestedTransformations(path string, rawData any) (any, error) {
	// If the data is not a map, no transformations to apply
	resultMap, isMap := rawData.(map[string]any)
	if !isMap {
		return d.applyTransformation(path)
	}

	// Create a copy of the map to avoid modifying the original
	resultCopy := make(map[string]any)
	for k, v := range resultMap {
		resultCopy[k] = v
	}

	// Look for transformations that should be applied to children
	for transformPath := range d.Transformations {
		if strings.HasPrefix(transformPath, path) {
			transformedValue, err := d.applyTransformation(transformPath)
			if err != nil {
				log.Printf("INFO: Failed to apply transformation for '%s': %s", transformPath, err.Error())
			}

			// Update the result with the transformed value
			subPathTokens := GetStrTokens(transformPath, path, "/")
			SetMapData(&resultCopy, subPathTokens, transformedValue)
		}
	}

	return resultCopy, nil
}

// GetModelData gets data from the model, applying transformations as needed
func (d *DataModel) GetModelData(pathTokens []string, raw bool) (any, error) {
	rawData, err := GetMapData(&d.Model, pathTokens)
	if raw && err != nil {
		// Ok if not raw since it might be a synthetic data point
		return nil, err
	}

	if raw {
		return rawData, nil
	}

	path := strings.Join(pathTokens, "/")

	transformedData, err := d.applyNestedTransformations(path, rawData)
	if err != nil {
		return nil, err
	}

	return transformedData, nil
}

// SetModelData sets data in the model without applying transformations
func (d *DataModel) SetModelData(pathTokens []string, value any) error {
	// Clear the transformation cache since model data is changing
	d.ClearCache(strings.Join(pathTokens, "/"))
	return SetMapData(&d.Model, pathTokens, value)
}
