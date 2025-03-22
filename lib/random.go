package lib

import (
	"math/rand"
)

func randomJitter() bool {
	return rand.Intn(100) > 10
}
