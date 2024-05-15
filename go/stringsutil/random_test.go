package stringsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomString(t *testing.T) {
	assert.Len(t, RandomString(10), 10)
}

func TestRandomToken64(t *testing.T) {
	assert.Len(t, RandomToken68(10), 10)
	assert.Len(t, RandomToken68(32), 32)
}

func BenchmarkRandomString(b *testing.B) {
	length := 512

	b.Run("RandomToken68", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if len(RandomToken68(length)) != length {
				b.Fail()
			}
		}
	})

	b.Run("RandomString", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if len(RandomString(length)) != length {
				b.Fail()
			}
		}
	})
}
