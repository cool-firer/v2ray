// +build !confonly

package core

import (
	"bytes"
	"context"

	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/features/routing"
	"v2ray.com/core/transport/internet/udp"
)

// CreateObject creates a new object based on the given V2Ray instance and config. The V2Ray instance may be nil.
func CreateObject(v *Instance, config interface{}) (interface{}, error) {
	// 奇怪的代码
	ctx := v.ctx
	if v != nil {
		// v2rayKey: 1
		ctx = context.WithValue(ctx, v2rayKey, v)
	}

	/**
		Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
		config proto结构体:
		{
			error_log_type: Console,
			error_log_level: Warning
		}

		Type: v2ray.core.app.dispatcher.Config, settings: 空的
		config proto结构体

		Type: v2ray.core.app.proxyman.InboundConfig, settings: 空的
		config proto结构体

		Type: v2ray.core.app.proxyman.InboundConfig settings: 空
		config proto结构体
	  返回 m := &Manager{
		taggedHandlers: make(map[string]inbound.Handler),
		}

		Type: v2ray.core.app.proxyman.OutboundConfig settings: 空
		config proto结构体


	config:
	core.InboundHandlerConfig proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto
	{
			Tag: ''
			ReceiverSettings: {
					type:"v2ray.core.app.proxyman.ReceiverConfig",

					value是 proto结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
					value: { 
						PortRange: net.PortRange proto结构体 10086
					}
			},
			ProxySettings: {
				type: 'v2ray.core.proxy.vmess.inbound.Config',
				value:  最终的config(proto生成的结构体): {
								SecureEncryptionOnly: false
								User: [
									proto生成的protocol.User结构体 {
										Account: {
												type: 'v2ray.core.proxy.vemss.Account',
												value: 整个Account二进制 { 只有id: 传入的id }
										} 
									},
								]
				}
			}
	}

	core.OutboundHandlerConfig: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto 生成的结构体
	{
			SenderSettings: {
				type: 'v2ray.core.app.proxyman.SenderConfig',
				value: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto 空的proto结构体
			},
			tag: '',
			ProxySettings: {
				type: 'v2ray.core.proxy.freedom.Config',
				value: {  /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/config.proto proto结构体
					DomainStrategy: freedom.Config_AS_IS 枚举值,
					UserLevel: 0,
				}
			}
	}

	*/
	return common.CreateObject(ctx, config)
}

// StartInstance starts a new V2Ray instance with given serialized config.
// By default V2Ray only support config in protobuf format, i.e., configFormat = "protobuf". Caller need to load other packages to add JSON support.
//
// v2ray:api:stable
func StartInstance(configFormat string, configBytes []byte) (*Instance, error) {
	config, err := LoadConfig(configFormat, "", bytes.NewReader(configBytes))
	if err != nil {
		return nil, err
	}
	instance, err := New(config)
	if err != nil {
		return nil, err
	}
	if err := instance.Start(); err != nil {
		return nil, err
	}
	return instance, nil
}

// Dial provides an easy way for upstream caller to create net.Conn through V2Ray.
// It dispatches the request to the given destination by the given V2Ray instance.
// Since it is under a proxy context, the LocalAddr() and RemoteAddr() in returned net.Conn
// will not show real addresses being used for communication.
//
// v2ray:api:stable
func Dial(ctx context.Context, v *Instance, dest net.Destination) (net.Conn, error) {
	dispatcher := v.GetFeature(routing.DispatcherType())
	if dispatcher == nil {
		return nil, newError("routing.Dispatcher is not registered in V2Ray core")
	}
	r, err := dispatcher.(routing.Dispatcher).Dispatch(ctx, dest)
	if err != nil {
		return nil, err
	}
	var readerOpt net.ConnectionOption
	if dest.Network == net.Network_TCP {
		readerOpt = net.ConnectionOutputMulti(r.Reader)
	} else {
		readerOpt = net.ConnectionOutputMultiUDP(r.Reader)
	}
	return net.NewConnection(net.ConnectionInputMulti(r.Writer), readerOpt), nil
}

// DialUDP provides a way to exchange UDP packets through V2Ray instance to remote servers.
// Since it is under a proxy context, the LocalAddr() in returned PacketConn will not show the real address.
//
// TODO: SetDeadline() / SetReadDeadline() / SetWriteDeadline() are not implemented.
//
// v2ray:api:beta
func DialUDP(ctx context.Context, v *Instance) (net.PacketConn, error) {
	dispatcher := v.GetFeature(routing.DispatcherType())
	if dispatcher == nil {
		return nil, newError("routing.Dispatcher is not registered in V2Ray core")
	}
	return udp.DialDispatcher(ctx, dispatcher.(routing.Dispatcher))
}
