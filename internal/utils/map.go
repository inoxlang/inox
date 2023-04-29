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

func EqualMaps[K comparable, V any](m1 map[K]V, m2 map[K]V, eql func(v1, v2 V) bool) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || !eql(v1, v2) {
			return false
		}
	}
	return true
}
