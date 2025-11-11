package postgres

// SchemaVersion represents the current schema version
const SchemaVersion = 4

// GetSchemaVersion returns the current schema version
func GetSchemaVersion() int {
	return SchemaVersion
}
