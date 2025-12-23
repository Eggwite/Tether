package utils

// GetBool safely retrieves a boolean value from an interface{}.
// If the value is not a boolean, it returns false.
func GetBool(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}
