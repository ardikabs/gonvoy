package types

// HeaderRange defines an interface implement Range function for iterating over header values
// and populating them into a target
type HeaderRange interface {
	// Range calls f sequentially for each key and value present in the map.
	// If f returns false, range stops the iteration.
	// When there are multiple values of a key, f will be invoked multiple times with the same key and each value.

	Range(f func(key string, value string) bool)
}
