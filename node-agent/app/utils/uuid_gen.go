package utils

import "github.com/google/uuid"

// GenerateUUID generates a new UUID string
func GenerateUUID() string {
	return uuid.New().String()
}

// ParseUUID parses a UUID string
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
