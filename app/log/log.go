// +build !confonly

package log

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"context"
	"sync"
	"fmt"

	"v2ray.com/core/common"
	"v2ray.com/core/common/log"
)

// Instance is a log.Handler that handles logs.
type Instance struct {
	sync.RWMutex
	config       *Config
	accessLogger log.Handler // 只有Handle方法的 interface
	errorLogger  log.Handler
	active       bool
}

// New creates a new log.Instance based on the given config.
		/**

		ctx: context.Background.WithValue(type core.V2rayKey, val <not Stringer>)

		Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
		config proto结构体:
		{
			error_log_type: Console,
			error_log_level: Warning
		}
	*/
func New(ctx context.Context, config *Config) (*Instance, error) {
	g := &Instance{
		config: config,
		active: false,
	}
	log.RegisterHandler(g) // 设置引用

	// start logger instantly on inited
	// other modules would log during init
	if err := g.startInternal(); err != nil {
		return nil, err
	}

	newError("Logger started").AtDebug().WriteToLog()
	return g, nil
}

func (g *Instance) initAccessLogger() error {
	// None '';
	fmt.Printf("in initAccessLogger, AccessLogType:%s,  AccessLogPath:%s \n", g.config.AccessLogType, g.config.AccessLogPath)
	handler, err := createHandler(g.config.AccessLogType, HandlerCreatorOptions{
		Path: g.config.AccessLogPath,
	})
	if err != nil {
		return err
	}
	g.accessLogger = handler // nil
	return nil
}

func (g *Instance) initErrorLogger() error {
	// 'Console', ''
	fmt.Printf("in initErrorLogger, AccessLogType:%s,  AccessLogPath:%s \n", g.config.ErrorLogType, g.config.ErrorLogPath)
	/**
	handler结构体
&generalLogger{
		creator: logWriterCreator,
		buffer:  make(chan Message, 16),
		access:  semaphore.New(1),
		done:    done.New(),
	}
	*/
	handler, err := createHandler(g.config.ErrorLogType, HandlerCreatorOptions{
		Path: g.config.ErrorLogPath,
	})
	if err != nil {
		return err
	}
	g.errorLogger = handler
	return nil
}

// Type implements common.HasType.
func (*Instance) Type() interface{} {
	return (*Instance)(nil)
}

func (g *Instance) startInternal() error {
	g.Lock()
	defer g.Unlock()

	if g.active { // Start()方法调用之前就是active了, 直接return
		return nil
	}

	g.active = true

	if err := g.initAccessLogger(); err != nil {
		return newError("failed to initialize access logger").Base(err).AtWarning()
	}
	if err := g.initErrorLogger(); err != nil {
		return newError("failed to initialize error logger").Base(err).AtWarning()
	}

	return nil
}

// Start implements common.Runnable.Start().
func (g *Instance) Start() error {
	return g.startInternal()
}

// Handle implements log.Handler.
func (g *Instance) Handle(msg log.Message) {
	g.RLock()
	defer g.RUnlock()

	if !g.active {
		return
	}

	switch msg := msg.(type) {
	case *log.AccessMessage:
		if g.accessLogger != nil {
			g.accessLogger.Handle(msg)
		}
	case *log.GeneralMessage:
		if g.errorLogger != nil && msg.Severity <= g.config.ErrorLogLevel {
			g.errorLogger.Handle(msg)
		}
	default:
		// Swallow
	}
}

// Close implements common.Closable.Close().
func (g *Instance) Close() error {
	newError("Logger closing").AtDebug().WriteToLog()

	g.Lock()
	defer g.Unlock()

	if !g.active {
		return nil
	}

	g.active = false

	common.Close(g.accessLogger) // nolint: errcheck
	g.accessLogger = nil

	common.Close(g.errorLogger) // nolint: errcheck
	g.errorLogger = nil

	return nil
}

func init() {
		/**

		ctx: context.Background.WithValue(type core.V2rayKey, val <not Stringer>)

		Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
		config proto结构体:
		{
			error_log_type: Console,
			error_log_level: Warning
		}
	*/
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		fmt.Printf("  in log.go, ctx:%+v  config:%+v\n", ctx, config)
		return New(ctx, config.(*Config))
	}))
}
