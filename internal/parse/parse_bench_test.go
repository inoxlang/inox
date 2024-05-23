package parse

import "testing"

func BenchmarkParse(b *testing.B) {
	program := "["
	for i := 0; i < 3000; i++ {
		program += "11111,"
	}

	program += "]"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		n, err := ParseChunk(program, "")
		if err != nil || n == nil {
			b.Fatal("parsing error")
		}
	}
}
