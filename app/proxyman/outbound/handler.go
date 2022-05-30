package outbound

import (
	"context"
	"fmt"

	"v2ray.com/core"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/mux"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/features/outbound"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/features/stats"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tls"
	"v2ray.com/core/transport/pipe"
)

func getStatCounter(v *core.Instance, tag string) (stats.Counter, stats.Counter) {
	var uplinkCounter stats.Counter
	var downlinkCounter stats.Counter

	policy := v.GetFeature(policy.ManagerType()).(policy.Manager)
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundUplink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>uplink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			uplinkCounter = c
		}
	}
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundDownlink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>downlink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			downlinkCounter = c
		}
	}

	return uplinkCounter, downlinkCounter
}

// Handler is an implements of outbound.Handler.
type Handler struct {
	tag             string
	senderSettings  *proxyman.SenderConfig
	streamSettings  *internet.MemoryStreamConfig
	proxy           proxy.Outbound
	outboundManager outbound.Manager
	mux             *mux.ClientManager
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter
}

// NewHandler create a new Handler based on the given configuration.
/**

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

*/
func NewHandler(ctx context.Context, config *core.OutboundHandlerConfig) (outbound.Handler, error) {
	v := core.MustFromContext(ctx)
	uplinkCounter, downlinkCounter := getStatCounter(v, config.Tag)
	h := &Handler{
		tag:             config.Tag, // ''
		outboundManager: v.GetFeature(outbound.ManagerType()).(outbound.Manager), // 指回server的outbound属性
		uplinkCounter:   uplinkCounter, // 空
		downlinkCounter: downlinkCounter, // 空
	}

	if config.SenderSettings != nil {
		/**

		in outbound/handler.go, config.SenderSettings not nil,  
		config:
			sender_settings:{
				type:"v2ray.core.app.proxyman.SenderConfig"
			} 
			proxy_settings:{
				type:"v2ray.core.proxy.freedom.Config"
			}	  

		config.SenderSettings:
			type:"v2ray.core.app.proxyman.SenderConfig"
		*/

		fmt.Printf("  in outbound/handler.go, config.SenderSettings not nil,  config:%+v  config.SenderSettings:%+v\n", config, config.SenderSettings)
		senderSettings, err := config.SenderSettings.GetInstance() // 空的proto结构体
		if err != nil {
			return nil, err
		}
		switch s := senderSettings.(type) {
		case *proxyman.SenderConfig:
			h.senderSettings = s

			// 跟inbound一样, tcp
			/**
			mss: 
 			&MemoryStreamConfig{
				ProtocolName: 'tcp'
				ProtocolSettings: 空的Config struct{
					new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
					HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
					AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
				},
			}
			*/
			mss, err := internet.ToMemoryStreamConfig(s.StreamSettings)
			if err != nil {
				return nil, newError("failed to parse stream settings").Base(err).AtWarning()
			}
			h.streamSettings = mss
		default:
			return nil, newError("settings is not SenderConfig")
		}
	}

	/**
		此时的h:
		h := &Handler{
			tag: '',
			outboundManager: 指回server里features的outbound.Manager属性
			uplinkCounter: 空接口值
			downlinkCounter: 空接口值
			senderSettings: v2ray.core.app.proxyman.SenderConfig 空的proto结构体,
			streamSettings: &MemoryStreamConfig{
				ProtocolName: 'tcp'
				ProtocolSettings: 空的Config struct{
					new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
					HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
					AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
				},
			}
		}
	*/


	proxyConfig, err := config.ProxySettings.GetInstance()
	if err != nil {
		return nil, err
	}

	/**
		rawProxyHandler: 空值
		type Handler struct {
			policyManager policy.Manager: 指向features里的同类型
			dns           dns.Client: 指向features里的同类型
			config        *Config:  空的config
			/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go
		}
	*/
	rawProxyHandler, err := common.CreateObject(ctx, proxyConfig)
	if err != nil {
		return nil, err
	}

	proxyHandler, ok := rawProxyHandler.(proxy.Outbound)
	if !ok {
		return nil, newError("not an outbound handler")
	}

	// h.senderSettings不nil,  但MultiplexSettings肯定是nil
	if h.senderSettings != nil && h.senderSettings.MultiplexSettings != nil {
		fmt.Printf("in outbound/handler.go, senderSettings:%+v  h.senderSettings.MultiplexSettings:%+v\n", h.senderSettings, h.senderSettings.MultiplexSettings)
		config := h.senderSettings.MultiplexSettings
		if config.Concurrency < 1 || config.Concurrency > 1024 {
			return nil, newError("invalid mux concurrency: ", config.Concurrency).AtWarning()
		}
		h.mux = &mux.ClientManager{
			Enabled: h.senderSettings.MultiplexSettings.Enabled,
			Picker: &mux.IncrementalWorkerPicker{
				Factory: &mux.DialingWorkerFactory{
					Proxy:  proxyHandler,
					Dialer: h,
					Strategy: mux.ClientStrategy{
						MaxConcurrency: config.Concurrency,
						MaxConnection:  128,
					},
				},
			},
		}
	}
	
	/**
		此时的h:
		h := &Handler{
			tag: '',
			outboundManager: 指回server里features的outbound.Manager属性
			uplinkCounter: 空接口值
			downlinkCounter: 空接口值
			senderSettings: v2ray.core.app.proxyman.SenderConfig 空的proto结构体,
			streamSettings: &MemoryStreamConfig{
				ProtocolName: 'tcp'
				ProtocolSettings: 空的Config struct{
					new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
					HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
					AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
				},
			},

			proxy: &Handler struct {
				policyManager policy.Manager: 指向features里的同类型
				dns           dns.Client: 指向features里的同类型
				config        *Config:  空的config
				/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go
			},
		}
	*/
	
	h.proxy = proxyHandler
	return h, nil
}

// Tag implements outbound.Handler.
func (h *Handler) Tag() string {
	return h.tag
}

// Dispatch implements proxy.Outbound.Dispatch.
func (h *Handler) Dispatch(ctx context.Context, link *transport.Link) {
	fmt.Println("[Sub]        Dispatch /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/handler.go")
	if h.mux != nil && (h.mux.Enabled || session.MuxPreferedFromContext(ctx)) {
		if err := h.mux.Dispatch(ctx, link); err != nil {
			newError("failed to process mux outbound traffic").Base(err).WriteToLog(session.ExportIDToError(ctx))
			common.Interrupt(link.Writer)
		}
	} else {
		fmt.Println("[Sub]        in else")
		// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go
		if err := h.proxy.Process(ctx, link, h); err != nil {
			// Ensure outbound ray is properly closed.
			newError("failed to process outbound traffic").Base(err).WriteToLog(session.ExportIDToError(ctx))
			
			fmt.Println("[Sub] gorotine returned with err:", err, " Interrupt Witer")
			common.Interrupt(link.Writer)
		} else {
			fmt.Println("[Sub] gorotine returned, Close Writer")
			common.Must(common.Close(link.Writer))
		}
		// fmt.Println("[freedom] Interrupt Reader")
		common.Interrupt(link.Reader)
	}
}

// Address implements internet.Dialer.
func (h *Handler) Address() net.Address {
	if h.senderSettings == nil || h.senderSettings.Via == nil {
		return nil
	}
	return h.senderSettings.Via.AsAddress()
}

// Dial implements internet.Dialer.
func (h *Handler) Dial(ctx context.Context, dest net.Destination) (internet.Connection, error) {
	fmt.Println("          in Dial /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/handler.go")
	if h.senderSettings != nil {
		if h.senderSettings.ProxySettings.HasTag() {
			fmt.Println("          has Tag")
			tag := h.senderSettings.ProxySettings.Tag
			handler := h.outboundManager.GetHandler(tag)
			if handler != nil {
				newError("proxying to ", tag, " for dest ", dest).AtDebug().WriteToLog(session.ExportIDToError(ctx))
				ctx = session.ContextWithOutbound(ctx, &session.Outbound{
					Target: dest,
				})

				opts := pipe.OptionsFromContext(ctx)
				uplinkReader, uplinkWriter := pipe.New(opts...)
				downlinkReader, downlinkWriter := pipe.New(opts...)

				go handler.Dispatch(ctx, &transport.Link{Reader: uplinkReader, Writer: downlinkWriter})
				conn := net.NewConnection(net.ConnectionInputMulti(uplinkWriter), net.ConnectionOutputMulti(downlinkReader))

				if config := tls.ConfigFromStreamSettings(h.streamSettings); config != nil {
					tlsConfig := config.GetTLSConfig(tls.WithDestination(dest))
					conn = tls.Client(conn, tlsConfig)
				}

				return h.getStatCouterConnection(conn), nil
			}

			newError("failed to get outbound handler with tag: ", tag).AtWarning().WriteToLog(session.ExportIDToError(ctx))
		}

		if h.senderSettings.Via != nil {
			fmt.Println("          Via not nil")
			outbound := session.OutboundFromContext(ctx)
			if outbound == nil {
				outbound = new(session.Outbound)
				ctx = session.ContextWithOutbound(ctx, outbound)
			}
			outbound.Gateway = h.senderSettings.Via.AsAddress()
		}
	}

	conn, err := internet.Dial(ctx, dest, h.streamSettings)
	return h.getStatCouterConnection(conn), err
}

func (h *Handler) getStatCouterConnection(conn internet.Connection) internet.Connection {
	if h.uplinkCounter != nil || h.downlinkCounter != nil {
		return &internet.StatCouterConnection{
			Connection:   conn,
			ReadCounter:  h.downlinkCounter,
			WriteCounter: h.uplinkCounter,
		}
	}
	return conn
}

// GetOutbound implements proxy.GetOutbound.
func (h *Handler) GetOutbound() proxy.Outbound {
	return h.proxy
}

// Start implements common.Runnable.
func (h *Handler) Start() error {
	return nil
}

// Close implements common.Closable.
func (h *Handler) Close() error {
	common.Close(h.mux)
	return nil
}
