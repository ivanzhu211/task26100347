package vegscatter

import "testing"

func BenchmarkQuery(b *testing.B) {
	sc := scenarios()[0]
	terrain := testTerrain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Query(sc.layer, terrain, sc.big)
	}
}
