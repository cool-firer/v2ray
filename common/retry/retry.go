package retry // import "v2ray.com/core/common/retry"

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"time"
)

var (
	ErrRetryFailed = newError("all retry attempts failed")
)

// Strategy is a way to retry on a specific function.
type Strategy interface {
	// On performs a retry on a specific function, until it doesn't return any error.
	On(func() error) error
}

type retryer struct {
	totalAttempt int
	nextDelay    func() uint32
}

// On implements Strategy.On.
func (r *retryer) On(method func() error) error {
	attempt := 0
	accumulatedError := make([]error, 0, r.totalAttempt) // 50

	for attempt < r.totalAttempt {
		// 调用方法
		err := method()
		if err == nil {
			return nil
		}

		numErrors := len(accumulatedError) // 当前错误数

		// 跟上次错误不一样
		if numErrors == 0 || err.Error() != accumulatedError[numErrors-1].Error() {
			accumulatedError = append(accumulatedError, err)
		}
		// 按 100, 200, 300, 400 ...ms
		delay := r.nextDelay()
		time.Sleep(time.Duration(delay) * time.Millisecond)
		attempt++
	}
	return newError(accumulatedError).Base(ErrRetryFailed)
}

// Timed returns a retry strategy with fixed interval.
func Timed(attempts int, delay uint32) Strategy {
	return &retryer{
		totalAttempt: attempts,
		nextDelay: func() uint32 {
			return delay
		},
	}
}

func ExponentialBackoff(attempts int, delay uint32) Strategy {
	nextDelay := uint32(0)
	return &retryer{
		totalAttempt: attempts, // 50
		nextDelay: func() uint32 {
			r := nextDelay
			nextDelay += delay // delay: 100
			return r
		},
	}
}
