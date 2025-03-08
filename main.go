package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Server encapsulates the HTTP server and its dependencies
type Server struct {
	dataModel DataModel
}

// CreateServer creates a new server with the given data model
func CreateServer(dataModel DataModel) *Server {
	return &Server{
		dataModel: dataModel,
	}
}

func (s *Server) GetPathTokens(r *http.Request) []string {
	// Get the complete path (everything following /model)
	fullPath := r.URL.Path

	// Trim both leading and trailing slashes from the suffix
	trimmedPath := strings.Trim(fullPath, "/")

	// Split the path suffix into tokens
	pathTokens := []string{}
	if trimmedPath != "" {
		pathTokens = strings.Split(trimmedPath, "/")
	}

	return pathTokens
}

// GetPostData extracts and validates JSON data from a POST request.
// Returns the parsed data and an error if any part of the validation fails.
func (s *Server) GetPostData(w http.ResponseWriter, r *http.Request) (any, error) {
	// Check request method
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("method %s not allowed, only POST is supported", r.Method)
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("unsupported Content-Type: %s, only application/json is supported", contentType)
	}

	// Limit request body size to 1MB to prevent DOS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	// Parse the JSON data
	var jsonData any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&jsonData); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Validate data type
	switch v := jsonData.(type) {
	case string, float64, bool, nil, map[string]any, []any:
		// These types are allowed
		return jsonData, nil
	default:
		return nil, fmt.Errorf("unsupported JSON data type: %T", v)
	}
}

// HandlePost processes POST requests and updates the data model.
// Returns an error if the operation fails.
func (s *Server) HandlePost(w http.ResponseWriter, r *http.Request) error {
	pathTokens := s.GetPathTokens(r)
	jsonData, err := s.GetPostData(w, r)
	if err != nil {
		return err
	}

	// Log what we're trying to do
	log.Printf("Handling POST to path: %v with data: %v", pathTokens, jsonData)

	// Start with the model map
	subJson := s.dataModel.Model

	// If there are no path tokens, replace the entire model
	if len(pathTokens) == 0 {
		// Check if jsonData is a map before assigning
		modelMap, ok := jsonData.(map[string]any)
		if !ok {
			return fmt.Errorf("expected JSON object for root model update, got %T", jsonData)
		}

		s.dataModel.Model = modelMap
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
	current[lastToken] = jsonData

	return nil
}

// HandleGet processes GET requests and returns the requested data.
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) error {
	pathTokens := s.GetPathTokens(r)

	// Start with the entire model
	var result any = s.dataModel.Model

	// Navigate through the path to find the requested sub-object
	for _, token := range pathTokens {
		// Check if we're still dealing with a map
		currentMap, ok := result.(map[string]any)
		if !ok {
			return fmt.Errorf("path element '%s' does not point to an object", token)
		}

		// Try to get the next element
		nextElement, exists := currentMap[token]
		if !exists {
			return fmt.Errorf("path element '%s' not found", token)
		}

		// Update result to the next element
		result = nextElement
	}

	// Convert result to JSON
	jsonResponse, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling response: %w", err)
	}

	// Set content type and write response
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
	return nil
}

// ModelHandler handles requests to the model endpoint
func (s *Server) ModelHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Log the request
	log.Printf("%s request to %s", r.Method, r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		err = s.HandleGet(w, r)
		if err != nil {
			// Determine appropriate status code based on error
			statusCode := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				statusCode = http.StatusNotFound
			} else if strings.Contains(err.Error(), "does not point to") {
				statusCode = http.StatusBadRequest
			}
			http.Error(w, err.Error(), statusCode)
			return
		}

	case http.MethodPost:
		err = s.HandlePost(w, r)
		if err != nil {
			// Determine appropriate status code based on error
			statusCode := http.StatusBadRequest
			if strings.Contains(err.Error(), "method") {
				statusCode = http.StatusMethodNotAllowed
			} else if strings.Contains(err.Error(), "Content-Type") {
				statusCode = http.StatusUnsupportedMediaType
			}
			http.Error(w, err.Error(), statusCode)
			return
		}

		// Send a success response for POST requests
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		responseData := map[string]string{"status": "success"}
		json.NewEncoder(w).Encode(responseData)

	default:
		http.Error(w, fmt.Sprintf("Method %s not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

func main() {
	dataModel, err := InitDataModel()
	if err != nil {
		log.Fatal("Failed to initialize data model:", err)
		return
	}

	// Create a new server with the data model
	server := CreateServer(dataModel)

	// Register the handler using a method closure
	http.HandleFunc("/", server.ModelHandler)

	// Start the server on port 8080
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server error:", err)
		return
	}
}
