package utils

func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	mapCopy := make(map[K]V, len(m))

	for k, v := range m {
		mapCopy[k] = v
	}

	return mapCopy
}

func GetMapKeys[K comparable, V any](m map[K]V) []K {
	var keys []K

	for k := range m {
		keys = append(keys, k)
	}

	return keys
}
