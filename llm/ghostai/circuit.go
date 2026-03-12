package ghostai

import (
	"sync/atomic"
	"time"
)

// Circuit breaker states.
const (
	circuitClosed   int32 = 0 // Normal operation.
	circuitOpen     int32 = 1 // Engine disabled after repeated failures.
	circuitHalfOpen int32 = 2 // Testing: next request decides if we recover.
)

// CircuitBreaker protects against a broken engine dragging the app down.
// After maxFails consecutive failures, it "trips" and rejects all requests
// for cooldownMs. After the cooldown, the next request is a test:
// if it succeeds, the circuit closes; if it fails, it re-opens.
type CircuitBreaker struct {
	state      atomic.Int32
	failures   atomic.Int32
	lastTrip   atomic.Int64 // UnixMilli of last trip
	maxFails   int
	cooldownMs int64
	tracer     *Tracer
}

// NewCircuitBreaker creates a circuit breaker.
// maxFails=3 and cooldownMs=30000 (30s) are sensible defaults.
func NewCircuitBreaker(maxFails int, cooldownMs int64, tracer *Tracer) *CircuitBreaker {
	if maxFails <= 0 {
		maxFails = 3
	}
	if cooldownMs <= 0 {
		cooldownMs = 30000
	}
	return &CircuitBreaker{
		maxFails:   maxFails,
		cooldownMs: cooldownMs,
		tracer:     tracer,
	}
}

// Allow checks if a request should proceed.
// Returns nil if OK, ErrCircuitOpen if the circuit is tripped.
func (cb *CircuitBreaker) Allow() error {
	state := cb.state.Load()

	switch state {
	case circuitClosed:
		return nil

	case circuitOpen:
		// Check if cooldown has elapsed.
		elapsed := time.Now().UnixMilli() - cb.lastTrip.Load()
		if elapsed >= cb.cooldownMs {
			// Transition to half-open: let one request through.
			if cb.state.CompareAndSwap(circuitOpen, circuitHalfOpen) {
				if cb.tracer != nil {
					cb.tracer.Debug("circuit: half-open, testing next request")
				}
			}
			return nil
		}
		return ErrCircuitOpen

	case circuitHalfOpen:
		// Already testing, allow the request.
		return nil
	}

	return nil
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.failures.Store(0)
	prev := cb.state.Swap(circuitClosed)
	if prev != circuitClosed && cb.tracer != nil {
		cb.tracer.TraceCircuitReset()
	}
}

// RecordFailure records a failed operation. May trip the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	n := int(cb.failures.Add(1))
	if n >= cb.maxFails {
		cb.state.Store(circuitOpen)
		cb.lastTrip.Store(time.Now().UnixMilli())
		if cb.tracer != nil {
			cb.tracer.TraceCircuitTrip(n)
		}
	}
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.failures.Store(0)
	cb.state.Store(circuitClosed)
	if cb.tracer != nil {
		cb.tracer.TraceCircuitReset()
	}
}

// IsOpen returns true if the circuit is currently tripped.
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.state.Load() == circuitOpen
}

// State returns a human-readable state string.
func (cb *CircuitBreaker) State() string {
	switch cb.state.Load() {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	return int(cb.failures.Load())
}
