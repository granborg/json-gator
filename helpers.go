package main

import (
	"fmt"
	"strings"
)

func GetStrTokens(path string, ignorePrefix string, sep string) []string {
	trimmedPath := strings.TrimLeft(path, ignorePrefix)
	// Trim both leading and trailing slashes from the suffix
	trimmedPath = strings.Trim(trimmedPath, sep)

	// Split the path suffix into tokens
	pathTokens := []string{}
	if trimmedPath != "" {
		pathTokens = strings.Split(trimmedPath, "/")
	}

	return pathTokens
}

// Helper function to create a deep copy of a map
func DeepCopyMap(original map[string]any) map[string]any {
	copy := make(map[string]any)

	for k, v := range original {
		switch val := v.(type) {
		case map[string]any:
			copy[k] = DeepCopyMap(val)
		case []any:
			copySlice := make([]any, len(val))
			for i, item := range val {
				if m, ok := item.(map[string]any); ok {
					copySlice[i] = DeepCopyMap(m)
				} else {
					copySlice[i] = item
				}
			}
			copy[k] = copySlice
		default:
			copy[k] = v
		}
	}

	return copy
}

// GetMapData gets data directly from the model without applying transformations
func GetMapData(modelMap *map[string]any, pathTokens []string) (any, error) {
	// Start with the entire model
	var result any = *modelMap

	// Navigate through the path to find the requested sub-object
	for _, token := range pathTokens {
		// Check if we're still dealing with a map
		currentMap, ok := result.(map[string]any)
		if !ok {
			return currentMap, nil
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

// SetMapData indexes into a map based on the pathTokens and changes the value.
func SetMapData(modelMap *map[string]any, pathTokens []string, value any) error {
	// If there are no path tokens, replace the entire model
	if len(pathTokens) == 0 {
		// Check if value is a map before assigning
		newModelMap, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("expected JSON object for root model update, got %T", value)
		}

		// Update the pointer to point to the new map
		*modelMap = newModelMap
		return nil
	}

	// For all other cases, we're working with the current map
	subJson := *modelMap

	// Special case: if we only have one path token
	if len(pathTokens) == 1 {
		subJson[pathTokens[0]] = value
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
	current[lastToken] = value

	return nil
}
