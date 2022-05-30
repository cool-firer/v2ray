package internet

import (
	"context"

	"v2ray.com/core/common/net"
)

var (
	transportListenerCache = make(map[string]ListenFunc)
)

func RegisterTransportListener(protocol string, listener ListenFunc) error {
	if _, found := transportListenerCache[protocol]; found {
		return newError(protocol, " listener already registered.").AtError()
	}
	transportListenerCache[protocol] = listener
	return nil
}

type ConnHandler func(Connection) // 这是什么?

type ListenFunc func(ctx context.Context, address net.Address, port net.Port, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error)

type Listener interface {
	Close() error
	Addr() net.Addr
}

// address: 0.0.0.0, port: 10086,  settings: &MemoryStreamConfig{..}
func ListenTCP(ctx context.Context, address net.Address, port net.Port, settings *MemoryStreamConfig, handler ConnHandler) (Listener, error) {
	if settings == nil {
		s, err := ToMemoryStreamConfig(nil)
		if err != nil {
			return nil, newError("failed to create default stream settings").Base(err)
		}
		settings = s
	}

	if address.Family().IsDomain() && address.Domain() == "localhost" {
		address = net.LocalHostIP
	}

	if address.Family().IsDomain() {
		return nil, newError("domain address is not allowed for listening: ", address.Domain())
	}

	protocol := settings.ProtocolName // 'tcp'

	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/hub.go
	listenFunc := transportListenerCache[protocol] // 返回原生的

	if listenFunc == nil {
		return nil, newError(protocol, " listener not registered.").AtError()
	}

/**
		listenFunc: listen监听 => go一个Accept 之后返回l
		l = &Listener{
			listener: listener, // 原生的 net.Listener
			config:   tcpSettings, // 空的结构体
			addConn:  handler, // 回调
		}
*/
	listener, err := listenFunc(ctx, address, port, settings, handler)
	if err != nil {
		return nil, newError("failed to listen on address: ", address, ":", port).Base(err)
	}
	return listener, nil
}

// ListenSystem listens on a local address for incoming TCP connections.
//
// v2ray:api:beta
func ListenSystem(ctx context.Context, addr net.Addr, sockopt *SocketConfig) (net.Listener, error) {
	return effectiveListener.Listen(ctx, addr, sockopt)
}

// ListenSystemPacket listens on a local address for incoming UDP connections.
//
// v2ray:api:beta
func ListenSystemPacket(ctx context.Context, addr net.Addr, sockopt *SocketConfig) (net.PacketConn, error) {
	return effectiveListener.ListenPacket(ctx, addr, sockopt)
}
