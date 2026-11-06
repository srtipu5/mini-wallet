package utils

import "os"

// Getenv returns the environment variable key, or fallback if it is unset or empty.
func Getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
