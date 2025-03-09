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

func (s *Server) GetStrTokens(path string, ignorePrefix string) []string {
	trimmedPath := strings.TrimLeft(path, ignorePrefix)
	// Trim both leading and trailing slashes from the suffix
	trimmedPath = strings.Trim(trimmedPath, "/")

	// Split the path suffix into tokens
	pathTokens := []string{}
	if trimmedPath != "" {
		pathTokens = strings.Split(trimmedPath, "/")
	}

	return pathTokens
}

func (s *Server) GetPathTokens(r *http.Request, ignorePrefix string) []string {
	// Get the complete path (everything following /model)
	fullPath := r.URL.Path
	return s.GetStrTokens(fullPath, ignorePrefix)
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
func (s *Server) HandleModelPost(w http.ResponseWriter, r *http.Request) error {
	pathTokens := s.GetPathTokens(r, "/model")
	jsonData, err := s.GetPostData(w, r)
	if err != nil {
		return err
	}

	log.Printf("Handling POST to path: %v with data: %v", pathTokens, jsonData)
	err = s.dataModel.UpdateModelData(pathTokens, jsonData)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) HandleNodePost(w http.ResponseWriter, r *http.Request) error {
	pathTokens := s.GetPathTokens(r, "/node")
	if len(pathTokens) != 1 {
		return fmt.Errorf("expected only one item in the path, got %T", len(pathTokens))
	}

	normalizedPath := strings.Join(pathTokens, "/")
	paths, exists := s.dataModel.Nodes[normalizedPath]
	if !exists {
		return fmt.Errorf("no match in the nodes list for the path \"%s\"", normalizedPath)
	}

	jsonData, err := s.GetPostData(w, r)
	if err != nil {
		return err
	}

	for _, value := range paths {
		curTokens := s.GetStrTokens(value, "/")
		s.dataModel.UpdateModelData(curTokens, jsonData)
	}

	return nil
}

// HandleGet processes GET requests and returns the requested data.
func (s *Server) HandleModelGet(w http.ResponseWriter, r *http.Request) error {
	pathTokens := s.GetPathTokens(r, "/model")
	result, err := s.dataModel.GetModelData(pathTokens)
	if err != nil {
		return fmt.Errorf("error getting data: %w", err)
	}

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
		err = s.HandleModelGet(w, r)
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
		err = s.HandleModelPost(w, r)
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

// ModelHandler handles requests to the node endpoint
func (s *Server) NodeHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Log the request
	log.Printf("%s request to %s", r.Method, r.URL.Path)

	switch r.Method {
	case http.MethodPost:
		err = s.HandleNodePost(w, r)
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
	http.HandleFunc("/model", server.ModelHandler)
	http.HandleFunc("/model/", server.ModelHandler)
	http.HandleFunc("/node", server.NodeHandler)
	http.HandleFunc("/node/", server.NodeHandler)

	// Start the server on port 8080
	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server error:", err)
		return
	}
}
