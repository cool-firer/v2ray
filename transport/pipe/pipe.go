package pipe

import (
	"context"

	"v2ray.com/core/common/signal"
	"v2ray.com/core/common/signal/done"
	"v2ray.com/core/features/policy"
)

// Option for creating new Pipes.
type Option func(*pipeOption)

// WithoutSizeLimit returns an Option for Pipe to have no size limit.
func WithoutSizeLimit() Option {
	return func(opt *pipeOption) {
		opt.limit = -1
	}
}

// WithSizeLimit returns an Option for Pipe to have the given size limit.
func WithSizeLimit(limit int32) Option {
	return func(opt *pipeOption) {
		opt.limit = limit
	}
}

// DiscardOverflow returns an Option for Pipe to discard writes if full.
func DiscardOverflow() Option {
	return func(opt *pipeOption) {
		opt.discardOverflow = true
	}
}

// OptionsFromContext returns a list of Options from context.
func OptionsFromContext(ctx context.Context) []Option {
	var opt []Option

	bp := policy.BufferPolicyFromContext(ctx)

	// 应该是-1
	if bp.PerConnection >= 0 {
		opt = append(opt, WithSizeLimit(bp.PerConnection))
	} else {
		//
		opt = append(opt, WithoutSizeLimit())
	}

	return opt
}

// New creates a new Reader and Writer that connects to each other.
func New(opts ...Option) (*Reader, *Writer) {
	p := &pipe{
		/**
		signal.NewNotifier():
			&Notifier{
				c: make(chan struct{}, 1),
			}
		*/
		readSignal:  signal.NewNotifier(),
		writeSignal: signal.NewNotifier(),

		/**
		done.New():
 			&Instance{
				c: make(chan struct{}),
			}
		*/
		done:        done.New(),
		option: pipeOption{
			limit: -1,
		},
	}

	// func(opt *pipeOption) {
	// 	opt.limit = limit
	// }
	for _, opt := range opts {
		opt(&(p.option))
	}

	return &Reader{
			pipe: p,
		}, &Writer{
			pipe: p,
		}
}
