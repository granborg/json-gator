package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// extractPathTokens extracts path tokens from a URL path after removing the prefix
func extractPathTokens(path, prefix string) []string {
	return GetStrTokens(path, prefix, "/")
}

// readJSONBody reads and parses the JSON request body
func readJSONBody(w http.ResponseWriter, r *http.Request) (any, error) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("unsupported Content-Type: %s, only application/json is supported", contentType)
	}

	// Limit request body size to 1MB to prevent DOS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %w", err)
	}

	var jsonData any
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	return jsonData, nil
}

// sendJSONResponse sends a JSON response to the client
func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	return json.NewEncoder(w).Encode(data)
}

// sendErrorResponse sends an error response with the appropriate status code
func sendErrorResponse(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError

	// Determine appropriate status code based on error message
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		statusCode = http.StatusNotFound
	case strings.Contains(errMsg, "method"):
		statusCode = http.StatusMethodNotAllowed
	case strings.Contains(errMsg, "Content-Type"):
		statusCode = http.StatusUnsupportedMediaType
	case strings.Contains(errMsg, "does not point to") ||
		strings.Contains(errMsg, "invalid JSON"):
		statusCode = http.StatusBadRequest
	}

	http.Error(w, errMsg, statusCode)
}

// ModelHandler handles requests to the model endpoint
func (s *Server) ModelHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s request to %s", r.Method, r.URL.Path)

	pathTokens := extractPathTokens(r.URL.Path, "/model")

	switch r.Method {
	case http.MethodGet:
		result, err := s.dataModel.GetModelData(pathTokens)
		if err != nil {
			sendErrorResponse(w, fmt.Errorf("error getting data: %w", err))
			return
		}

		if err := sendJSONResponse(w, result, http.StatusOK); err != nil {
			sendErrorResponse(w, fmt.Errorf("error encoding response: %w", err))
		}

	case http.MethodPost:
		jsonData, err := readJSONBody(w, r)
		if err != nil {
			sendErrorResponse(w, err)
			return
		}

		if err := s.dataModel.UpdateModelData(pathTokens, jsonData); err != nil {
			sendErrorResponse(w, err)
			return
		}

		sendJSONResponse(w, map[string]string{"status": "success"}, http.StatusOK)

	default:
		sendErrorResponse(w, fmt.Errorf("method %s not supported", r.Method))
	}
}

// NodeHandler handles requests to the node endpoint
func (s *Server) NodeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s request to %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, fmt.Errorf("method %s not supported", r.Method))
		return
	}

	pathTokens := extractPathTokens(r.URL.Path, "/node")
	if len(pathTokens) != 1 {
		sendErrorResponse(w, fmt.Errorf("expected only one item in the path, got %d", len(pathTokens)))
		return
	}

	normalizedPath := strings.Join(pathTokens, "/")
	paths, exists := s.dataModel.Nodes[normalizedPath]
	if !exists {
		sendErrorResponse(w, fmt.Errorf("no match in the nodes list for the path \"%s\"", normalizedPath))
		return
	}

	jsonData, err := readJSONBody(w, r)
	if err != nil {
		sendErrorResponse(w, err)
		return
	}

	// Update all paths associated with this node
	for _, path := range paths {
		curTokens := GetStrTokens(path, "/", "/")
		if err := s.dataModel.UpdateModelData(curTokens, jsonData); err != nil {
			sendErrorResponse(w, err)
			return
		}
	}

	sendJSONResponse(w, map[string]string{"status": "success"}, http.StatusOK)
}

// ConfigHandler handles requests to the config endpoint
func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s request to %s", r.Method, r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		if err := sendJSONResponse(w, s.dataModel, http.StatusOK); err != nil {
			sendErrorResponse(w, fmt.Errorf("error encoding data model: %w", err))
		}

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			sendErrorResponse(w, fmt.Errorf("error reading request body: %w", err))
			return
		}
		defer r.Body.Close()

		var dataModel DataModel
		if err := json.Unmarshal(body, &dataModel); err != nil {
			sendErrorResponse(w, fmt.Errorf("error parsing JSON into DataModel: %w", err))
			return
		}

		if err := SaveDataModel(dataModel); err != nil {
			sendErrorResponse(w, fmt.Errorf("error saving data model: %w", err))
			return
		}

		// Update server's data model
		s.dataModel = dataModel

		sendJSONResponse(w, map[string]string{"status": "success"}, http.StatusOK)

	default:
		sendErrorResponse(w, fmt.Errorf("method %s not supported", r.Method))
	}
}

func main() {
	dataModel, err := LoadDataModel()
	if err != nil {
		log.Fatalf("Failed to initialize data model: %v", err)
	}

	server := CreateServer(dataModel)

	http.HandleFunc("/model", server.ModelHandler)
	http.HandleFunc("/model/", server.ModelHandler)
	http.HandleFunc("/node", server.NodeHandler)
	http.HandleFunc("/node/", server.NodeHandler)
	http.HandleFunc("/config", server.ConfigHandler)
	http.HandleFunc("/config/", server.ConfigHandler)

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port[1:])
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
