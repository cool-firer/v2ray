package common

import (
	"context"
	"reflect"

	"fmt"
)

// ConfigCreator is a function to create an object by a config.
type ConfigCreator func(ctx context.Context, config interface{}) (interface{}, error)

var (
	typeCreatorRegistry = make(map[reflect.Type]ConfigCreator)
)

// RegisterConfig registers a global config creator. The config can be nil but must have a type.
func RegisterConfig(config interface{}, configCreator ConfigCreator) error {
	configType := reflect.TypeOf(config)
	if _, found := typeCreatorRegistry[configType]; found {
		return newError(configType.Name() + " is already registered").AtError()
	}
	typeCreatorRegistry[configType] = configCreator
	return nil
}

// CreateObject creates an object by its config. The config type must be registered through RegisterConfig().
func CreateObject(ctx context.Context, config interface{}) (interface{}, error) {

	/**
		Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
		config proto结构体:
		{
			error_log_type: Console,
			error_log_level: Warning
		}
		typeCreatorRegistry在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/log/log.go 里注册

		Type: v2ray.core.app.dispatcher.Config, settings: 空
		config proto空结构体
		typeCreatorRegistry在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/dispatcher/default.go
		这个config typeCreatorRegistry里面会动态添加server属性

		Type: v2ray.core.app.proxyman.InboundConfig settings: 空
		config proto结构体
		typeCreatorRegistry在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/inbound.go

		Type: v2ray.core.app.proxyman.OutboundConfig settings: 空
		config proto结构体
		typeCreatorRegistry在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/outbound.go


		core.InboundHandlerConfig proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto
		typeCreatorRegistry 

		configType *inbound.Config

	*/
	configType := reflect.TypeOf(config) 
	fmt.Printf("  configType %+v\n", configType) // *log.Config *dispatcher.Config 	*proxyman.InboundConfig

	// *core.OutboundHandlerConfig
	creator, found := typeCreatorRegistry[configType]
	if !found {
		return nil, newError(configType.String() + " is not registered").AtError()
	}
	return creator(ctx, config)
}
