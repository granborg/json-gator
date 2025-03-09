package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func initDataModelFromFile(path string) (DataModel, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		// Initialize with empty maps when no source is provided
		return DataModel{
			Model:           make(map[string]any),
			Transformations: make(map[string]any),
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
			Transformations: make(map[string]any),
			Nodes:           make(map[string][]string),
		}, err
	}

	return dataModel, nil
}

// LoadDataModel initializes a data model from the file path CONFIG_FILE_PATH, or config.json if empty.
func LoadDataModel() (DataModel, error) {
	path := os.Getenv("CONFIG_FILE_PATH")
	if path != "" {
		return initDataModelFromFile(path)
	}

	return initDataModelFromFile("config.json")
}

// SaveDataModel saves the model to the file path CONFIG_FILE_PATH, or config.json if empty.
func SaveDataModel(dataModel DataModel) error {
	path := os.Getenv("CONFIG_FILE_PATH")
	if path == "" {
		path = "config.json"
	}

	data, err := json.Marshal(dataModel)
	if err != nil {
		return fmt.Errorf("error marshaling data model to JSON: %s", err.Error())
	}

	return os.WriteFile(path, data, 0644)
}
