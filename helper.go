package envoy

// MustGetProperty is an extended of GetProperty, only panic if value is not in acceptable format.
func MustGetProperty(c Context, name, defaultVal string) string {
	value, err := c.GetProperty(name, defaultVal)
	if err != nil {
		panic(err)
	}

	return value
}
