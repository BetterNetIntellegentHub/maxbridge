package delivery

import "testing"

func TestNextBackoffMonotonic(t *testing.T) {
	b1 := NextBackoff(1)
	b2 := NextBackoff(2)
	if b2 <= b1/2 {
		t.Fatalf("unexpected backoff progression: b1=%v b2=%v", b1, b2)
	}
}
