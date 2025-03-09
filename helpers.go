package main

import "strings"

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
