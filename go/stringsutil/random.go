package stringsutil

import (
	"math/rand"
	"strings"
)

var chars = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890")

func RandomString(length int) string {
	var b strings.Builder
	b.Grow(length)
	for range length {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

var token68 = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/")

func RandomToken68(length int) string {
	var b strings.Builder
	b.Grow(length)
	for range length {
		b.WriteRune(token68[rand.Intn(len(chars))])
	}
	return b.String()
}
