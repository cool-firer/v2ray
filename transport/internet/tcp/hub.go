// +build !confonly

package tcp

import (
	"context"
	gotls "crypto/tls"
	"strings"
	"time"

	"github.com/pires/go-proxyproto"
	goxtls "github.com/xtls/go"

	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tls"
	"v2ray.com/core/transport/internet/xtls"
)

// Listener is an internet.Listener that listens for TCP connections.
type Listener struct {
	listener   net.Listener
	tlsConfig  *gotls.Config
	xtlsConfig *goxtls.Config
	authConfig internet.ConnectionAuthenticator
	config     *Config
	addConn    internet.ConnHandler
}

// ListenTCP creates a new Listener based on configurations.
/**
	adderss: 0.0.0.0
	port: 10086
	streamSettings: &MemoryStreamConfig{
											ProtocolName: 'tcp',
											ProtocolSettings: &Config struct { new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
											HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
											AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
										},
	handler: 一个回调func
**/
func ListenTCP(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, handler internet.ConnHandler) (internet.Listener, error) {
	
	// 内置包的 Listen方法, 返回 原生的Listner
	listener, err := internet.ListenSystem(ctx, &net.TCPAddr{
		IP:   address.IP(),
		Port: int(port), // 10086
	}, streamSettings.SocketSettings)

	if err != nil {
		return nil, newError("failed to listen TCP on", address, ":", port).Base(err)
	}
	newError("listening TCP on ", address, ":", port).WriteToLog(session.ExportIDToError(ctx))

	tcpSettings := streamSettings.ProtocolSettings.(*Config)

	var l *Listener

	// 应该是nil, 确实是
	if tcpSettings.AcceptProxyProtocol {
		newError("AcceptProxyProtocol not nil").AtWarning().WriteToLog()
		policyFunc := func(upstream net.Addr) (proxyproto.Policy, error) { return proxyproto.REQUIRE, nil }
		l = &Listener{
			listener: &proxyproto.Listener{Listener: listener, Policy: policyFunc},
			config:   tcpSettings,
			addConn:  handler,
		}
		newError("accepting PROXY protocol").AtWarning().WriteToLog(session.ExportIDToError(ctx))
	} else {
		newError("AcceptProxyProtocol nil").AtWarning().WriteToLog()
		l = &Listener{
			listener: listener, // 原生的 net.Listener
			config:   tcpSettings, // 空的结构体
			addConn:  handler, // 回调
		}
	}

	if config := tls.ConfigFromStreamSettings(streamSettings); config != nil {
		l.tlsConfig = config.GetTLSConfig(tls.WithNextProto("h2"))
	}
	if config := xtls.ConfigFromStreamSettings(streamSettings); config != nil {
		l.xtlsConfig = config.GetXTLSConfig(xtls.WithNextProto("h2"))
	}

	if tcpSettings.HeaderSettings != nil {
		headerConfig, err := tcpSettings.HeaderSettings.GetInstance()
		if err != nil {
			return nil, newError("invalid header settings").Base(err).AtError()
		}
		auth, err := internet.CreateConnectionAuthenticator(headerConfig)
		if err != nil {
			return nil, newError("invalid header settings.").Base(err).AtError()
		}
		l.authConfig = auth
	}
	
	go l.keepAccepting()
	return l, nil
}

func (v *Listener) keepAccepting() {
	// 按照常规, 应该是Accept之后再起一个 goroutine, 这里竟然是在Accept起go??
	for {
		conn, err := v.listener.Accept() // 终于看到了 accept
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "closed") {
				break
			}
			newError("failed to accepted raw connections").Base(err).AtWarning().WriteToLog()
			if strings.Contains(errStr, "too many") {
				time.Sleep(time.Millisecond * 500)
			}
			continue
		}

		if v.tlsConfig != nil {
			conn = tls.Server(conn, v.tlsConfig)
		} else if v.xtlsConfig != nil {
			conn = xtls.Server(conn, v.xtlsConfig)
		}
		if v.authConfig != nil {
			conn = v.authConfig.Server(conn)
		}

		// addConn是个空函数？
		// 哦..... addConn是传进来的参数
		v.addConn(internet.Connection(conn)) // 相当于转成 internet.Connection 接口
	}
}

// Addr implements internet.Listener.Addr.
func (v *Listener) Addr() net.Addr {
	return v.listener.Addr()
}

// Close implements internet.Listener.Close.
func (v *Listener) Close() error {
	return v.listener.Close()
}

func init() {
	common.Must(internet.RegisterTransportListener(protocolName, ListenTCP))
}
