package internet

import (
	"context"
	"fmt"

	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
)

// Dialer is the interface for dialing outbound connections.
type Dialer interface {
	// Dial dials a system connection to the given destination.
	Dial(ctx context.Context, destination net.Destination) (Connection, error)

	// Address returns the address used by this Dialer. Maybe nil if not known.
	Address() net.Address
}

// dialFunc is an interface to dial network connection to a specific destination.
type dialFunc func(ctx context.Context, dest net.Destination, streamSettings *MemoryStreamConfig) (Connection, error)

var (
	transportDialerCache = make(map[string]dialFunc)
)

// RegisterTransportDialer registers a Dialer with given name.
func RegisterTransportDialer(protocol string, dialer dialFunc) error {
	if _, found := transportDialerCache[protocol]; found {
		return newError(protocol, " dialer already registered").AtError()
	}
	transportDialerCache[protocol] = dialer
	return nil
}

// Dial dials a internet connection towards the given destination.

/**
&MemoryStreamConfig{
	ProtocolName: 'tcp'
	ProtocolSettings: 空的Config struct{
			new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
			HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
			AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
	},
},
*/
func Dial(ctx context.Context, dest net.Destination, streamSettings *MemoryStreamConfig) (Connection, error) {
	if dest.Network == net.Network_TCP {
		if streamSettings == nil {
			s, err := ToMemoryStreamConfig(nil)
			if err != nil {
				return nil, newError("failed to create default stream settings").Base(err)
			}
			streamSettings = s
		}

		protocol := streamSettings.ProtocolName // 'tcp'
		dialer := transportDialerCache[protocol]
		if dialer == nil {
			return nil, newError(protocol, " dialer not registered").AtError()
		}
		fmt.Println("dest.Network == net.Network_TCP")
		///Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/dialer.go
		return dialer(ctx, dest, streamSettings)
	}

	if dest.Network == net.Network_UDP {
		udpDialer := transportDialerCache["udp"]
		if udpDialer == nil {
			return nil, newError("UDP dialer not registered").AtError()
		}
		return udpDialer(ctx, dest, streamSettings)
	}

	return nil, newError("unknown network ", dest.Network)
}

// DialSystem calls system dialer to create a network connection.
func DialSystem(ctx context.Context, dest net.Destination, sockopt *SocketConfig) (net.Conn, error) {
	var src net.Address
	if outbound := session.OutboundFromContext(ctx); outbound != nil {
		src = outbound.Gateway
	}
	return effectiveSystemDialer.Dial(ctx, src, dest, sockopt)
}
