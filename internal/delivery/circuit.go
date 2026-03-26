package delivery

import (
	"sync"
	"time"
)

type CircuitBreaker struct {
	mu                  sync.Mutex
	consecutiveFailures int
	openUntil           time.Time
	threshold           int
	cooldown            time.Duration
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().After(c.openUntil)
}

func (c *CircuitBreaker) MarkSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutiveFailures = 0
	c.openUntil = time.Time{}
}

func (c *CircuitBreaker) MarkFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutiveFailures++
	if c.consecutiveFailures >= c.threshold {
		c.openUntil = time.Now().Add(c.cooldown)
		c.consecutiveFailures = 0
	}
}
