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
		dataModel := NewDataModel()
		return *dataModel, err
	}

	dataModel := NewDataModel()

	err = json.Unmarshal(content, &dataModel)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		// Initialize with empty maps when no source is provided
		return *dataModel, err
	}

	return *dataModel, nil
}

// LoadDataModel initializes a data model from the file path CONFIG_FILE_PATH, or config.json if empty.
func LoadDataModel() (DataModel, error) {
	path := os.Getenv("CONFIG_FILE_PATH")
	if path != "" {
		return initDataModelFromFile(path)
	}

	dataModel, err := initDataModelFromFile("config.json")
	if err != nil {
		return dataModel, err
	}

	if dataModel.Mqtt != nil {
		dataModel.Mqtt.Connect(nil)
	}

	return dataModel, err
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
