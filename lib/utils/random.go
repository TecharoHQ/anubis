package utils

import (
	"math/rand"
)

func RandomJitter() bool {
	return rand.Intn(100) > 10
}
