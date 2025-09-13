package stringsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomStringWithCharset(t *testing.T) {
	assert.Len(t, RandomStringWithCharset(10, []rune("ABCDEFG")), 10)
}

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
		for range b.N {
			if len(RandomToken68(length)) != length {
				b.Fail()
			}
		}
	})

	b.Run("RandomString", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if len(RandomString(length)) != length {
				b.Fail()
			}
		}
	})
}
