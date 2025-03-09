package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	v8 "rogchap.com/v8go"
)

type DataModel struct {
	Model           map[string]any      `json:"model"`           // Contains the current values for all topics
	Transformations map[string]string   `json:"transformations"` // key: topic, value: transformation
	Nodes           map[string][]string `json:"nodes"`           // key: datum ID, value: all associated topics
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

func (d *DataModel) ApplyTransformation(pathTokens []string, data any) (any, error) {
	path := strings.Join(pathTokens, "/")
	transformation, exists := d.Transformations[path]
	if !exists {
		// This is ok.
		return data, fmt.Errorf("did not find a transformation with the path \"%s\"", path)
	}

	// Create the V8 environment
	iso := v8.NewIsolate()
	defer iso.Dispose() // Don't forget to dispose the isolate

	// Create the global object template
	global := v8.NewObjectTemplate(iso)

	// Create the context with the global template
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	// Convert Go data to JavaScript object
	jsInput, err := convertGoToJavaScript(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data to JavaScript object: %s", err.Error())
	}

	// Set the "self" variable in the global context BEFORE running any script
	err = ctx.Global().Set("self", jsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to set 'self' in global context: %s", err.Error())
	}

	// Now run the transformation script
	jsResult, err := ctx.RunScript(transformation, "transformation.js")
	if err != nil {
		return nil, fmt.Errorf("failed to execute JavaScript: %s", err.Error())
	}

	result, err := convertJavaScriptToGo(ctx, jsResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JavaScript result \"%s\" to Go object: %s", jsResult, err.Error())
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

	result, err := d.ApplyTransformation(pathTokens, jsonData)
	if err != nil {
		log.Printf("Transformation failed with error: %s", err.Error())
	}

	current[lastToken] = result
	return nil
}

func initDataModelFromFile(path string) (DataModel, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}

	var dataModel DataModel
	err = json.Unmarshal(content, &dataModel)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}

	return dataModel, nil
}

func initDataModelFromUrl(url string) (DataModel, error) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching config from URL:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
		fmt.Println("Error fetching config from URL:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}

	var dataModel DataModel
	err = json.Unmarshal(content, &dataModel)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]string),
			Nodes:           make(map[string][]string),
		}, err
	}

	return dataModel, nil
}

func convertGoToJavaScript(ctx *v8.Context, goObj any) (*v8.Value, error) {
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

func convertJavaScriptToGo(ctx *v8.Context, jsValue *v8.Value) (any, error) {
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

// InitDataModel initializes a data model from a file, URL, or other source depending on ENV file configuration.
// It checks env variables in this order: CONFIG_FILE_PATH, CONFIG_FILE_URL.
func InitDataModel() (DataModel, error) {
	path := os.Getenv("CONFIG_FILE_PATH")
	if path != "" {
		return initDataModelFromFile(path)
	}

	url := os.Getenv("CONFIG_FILE_URL")
	if url != "" {
		return initDataModelFromUrl(url)
	}

	return initDataModelFromFile("config.json")
}
