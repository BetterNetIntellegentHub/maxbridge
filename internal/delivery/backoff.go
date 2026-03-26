package delivery

import (
	"math"
	"math/rand"
	"time"
)

func NextBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exp := uint(minInt(attempt, 8))
	pow := 1 << exp
	base := float64(pow)
	sec := math.Min(base, 300)
	jitter := 0.8 + rand.Float64()*0.4
	return time.Duration(sec*jitter) * time.Second
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}