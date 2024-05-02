package utils

func HasKeys[K string, V any](m map[K]V, keys ...K) bool {
	for _, key := range keys {
		_, ok := m[key]
		if !ok {
			return false
		}
	}
	return true
}

func SameKeys[K comparable, V any](m1 map[K]V, m2 map[K]V) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k := range m1 {
		_, ok := m2[k]
		if !ok {
			return false
		}
	}
	return true
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

func KeySet[K comparable, V any](m map[K]V) map[K]struct{} {
	_map := make(map[K]struct{}, len(m))
	for k := range m {
		_map[k] = struct{}{}
	}
	return _map
}
