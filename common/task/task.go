package task

import (
	"context"

	"v2ray.com/core/common/signal/semaphore"
)

// OnSuccess executes g() after f() returns nil.
func OnSuccess(f func() error, g func() error) func() error {
	return func() error {
		
		if err := f(); err != nil {
			return err
		}
		return g()
	}
}

// Run executes a list of tasks in parallel, returns the first error encountered or nil if all tasks pass.
func Run(ctx context.Context, tasks ...func() error) error {
	n := len(tasks) // 2

	/**
	&Instance{
		token: make(chan struct{}, 2), 存两个空值
	}
	*/
	s := semaphore.New(n)

	done := make(chan error, 1)

	for _, task := range tasks {
		<-s.Wait() // 尝试读取一个
		go func(f func() error) {
			err := f()
			if err == nil {
				s.Signal() // 放个值
				return
			}

			select {
			case done <- err:
			default:
			}
		}(task)
	}

	for i := 0; i < n; i++ {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-s.Wait():
		}
	}

	return nil
}
