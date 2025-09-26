package util

import "os"

// FileExists checks if a file exists and returns true if it does, false otherwise
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
