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
	if attempt > 8 {
		attempt = 8
	}
	pow := 1 << attempt
	base := float64(pow)
	sec := math.Min(base, 300)
	// #nosec G404 -- jitter only needs non-crypto randomness.
	jitter := 0.8 + rand.Float64()*0.4
	return time.Duration(sec*jitter) * time.Second
}
