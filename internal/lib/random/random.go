package random

import "math/rand"

func String(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz1234567890"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}
