package utils

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("json")
		if name == "" {
			return fld.Name
		}
		return name
	})
}

// ValidateStruct validates a struct using go-playground/validator
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		validationErrors := make(map[string]string)
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors[err.Field()] = fmt.Sprintf("validation failed: %s", err.Tag())
		}
		return fmt.Errorf("validation failed: %v", validationErrors)
	}
	return nil
}

// ValidateCommandPayload validates a command payload against its type
func ValidateCommandPayload(commandType string, payload map[string]interface{}) error {
	schema, exists := getCommandRegistry()[commandType]
	if !exists {
		return fmt.Errorf("unknown command type: %s", commandType)
	}

	// Create a new instance of the schema type
	schemaType := reflect.TypeOf(schema)
	instance := reflect.New(schemaType).Interface()

	// Convert map to JSON bytes and unmarshal into the instance
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, instance); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Validate the instance
	return ValidateStruct(instance)
}

// getCommandRegistry returns the command registry (imported from dto package)
func getCommandRegistry() map[string]interface{} {
	// This will be imported from dto package
	// For now, we'll define it here to avoid circular imports
	return map[string]interface{}{
		"RunCommand": struct {
			Cmd        string   `json:"cmd" validate:"required"`
			Args       []string `json:"args,omitempty"`
			TimeoutSec int      `json:"timeout_sec,omitempty"`
		}{},
		"UpdateAgent": struct {
			Version string `json:"version" validate:"required"`
			URL     string `json:"url" validate:"required,url"`
		}{},
		"UpdatePackage": struct {
			Packages []string `json:"packages" validate:"required"`
			Action   string   `json:"action" validate:"required,oneof=install remove upgrade"`
		}{},
	}
}
