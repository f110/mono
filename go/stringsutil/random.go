package stringsutil

import (
	"math/rand"
	"strings"
)

var chars = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890")

func RandomString(length int) string {
	return RandomStringWithCharset(length, chars)
}

var token68 = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/")

func RandomToken68(length int) string {
	return RandomStringWithCharset(length, token68)
}

func RandomStringWithCharset(length int, charset []rune) string {
	var b strings.Builder
	b.Grow(length)
	for range length {
		b.WriteRune(charset[rand.Intn(len(charset))])
	}
	return b.String()
}
