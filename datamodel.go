package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type DataModel struct {
	Model           map[string]any      `json:"model"`           // Contains the current values for all topics
	Transformations map[string]string   `json:"transformations"` // key: topic, value: transformation
	Nodes           map[string][]string `json:"nodes"`           // key: datum ID, value: all associated topics
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
