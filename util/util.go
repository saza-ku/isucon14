package util

// S2m converts a slice to a map.
func S2m[T any, K comparable](s []T, proj func(T) K) map[K]T {
	m := map[K]T{}

	for _, v := range s {
		m[proj(v)] = v
	}
	return m
}
