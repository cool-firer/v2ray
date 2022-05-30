package inbound

import (
	"context"
	"fmt"

	"v2ray.com/core"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/mux"
	"v2ray.com/core/common/net"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/features/stats"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet"
)

func getStatCounter(v *core.Instance, tag string) (stats.Counter, stats.Counter) {
	var uplinkCounter stats.Counter
	var downlinkCounter stats.Counter

	// 找到eatures里的 同类型值
	policy := v.GetFeature(policy.ManagerType()).(policy.Manager)

	// tag是 ''
	if len(tag) > 0 && policy.ForSystem().Stats.InboundUplink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "inbound>>>" + tag + ">>>traffic>>>uplink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			uplinkCounter = c
		}
	}
	// tag是 ''
	if len(tag) > 0 && policy.ForSystem().Stats.InboundDownlink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "inbound>>>" + tag + ">>>traffic>>>downlink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			downlinkCounter = c
		}
	}

	return uplinkCounter, downlinkCounter
}

type AlwaysOnInboundHandler struct {
	proxy   proxy.Inbound
	workers []worker
	mux     *mux.Server
	tag     string
}

/**
	{
			Tag: ''
			receiverConfig: {
					type:"v2ray.core.app.proxyman.ReceiverConfig",

					value是 proto结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
					value: { 
						PortRange: net.PortRange proto结构体 10086
					}
			},

			proxyConfig: {
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

*/
func NewAlwaysOnInboundHandler(ctx context.Context, tag string, receiverConfig *proxyman.ReceiverConfig, proxyConfig interface{}) (*AlwaysOnInboundHandler, error) {
	
	// configType *inbound.Config
	/**
	在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/inbound.go 
	rawProxy: handler := &Handler{
			policyManager: 指向server里features里的同类,
			inboundHandlerManager:  指向server里features里的同类,

			clients: &TimedUserValidator{
				users: [
					&user{
						user: *mUser,
						lastSec: protocol.Timestamp(nowSec - cacheDurationSec),
					},
				],
				userHash: make(map[[16]byte]indexTimePair, 1024), 有值
				hasher:  一个func: protocol.DefaultIDHash,
				baseTime: protocol.Timestamp(time.Now().Unix() - cacheDurationSec*2),
				aeadDecoderHolder: aead.NewAuthIDDecoderHolder(), 有值
			},

			detours: nil,
			usersByEmail: &userByEmail{
				cache:           make(map[string]*protocol.MemoryUser),
				defaultLevel:    0,
				defaultAlterIDs: 32,
			},
			sessionHistory: encoding.NewSessionHistory(),
			secure:          false
	}

	configType *http.ServerConfig
	在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/server.go
	rawProxy: s := &Server{ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/server.go
		config: 空结构体,
		policyManager: 指向features的*policy.Manager
	}
	*/
	rawProxy, err := common.CreateObject(ctx, proxyConfig)
	if err != nil {
		return nil, err
	}
	p, ok := rawProxy.(proxy.Inbound)
	if !ok {
		return nil, newError("not an inbound proxy.")
	}

	fmt.Print("  initilized rawProxy\n\n")
	
	h := &AlwaysOnInboundHandler{
		proxy: p,

		/**
			&Server{
				dispatcher: 指向 features 里的 *routing.Dispatcher 实例
			}
		*/
		mux:   mux.NewServer(ctx),
		tag:   tag, // ''
	}

	fmt.Print("  initilized h\n\n")

	// 	var uplinkCounter stats.Counter
	//  var downlinkCounter stats.Counter
	// 两个空的接口
	uplinkCounter, downlinkCounter := getStatCounter(core.MustFromContext(ctx), tag)

	/**
		p.Network()函数
		// type Network int32
		func (*Handler) Network() []net.Network {
			return []net.Network{net.Network_TCP}
		}
	*/
	nl := p.Network() // 相当于一个[]int 切片

	pr := receiverConfig.PortRange

	// 应该是nil
	address := receiverConfig.Listen.AsAddress()
	if address == nil {
		address = net.AnyIP
	}

	/**
		mss := &MemoryStreamConfig{
			ProtocolName: 'tcp',
			ProtocolSettings: &Config struct { new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
				HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
				AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
			}
		}
	*/
	fmt.Println("  internet.ToMemoryStreamConfig...receiverConfig.StreamSettings:", receiverConfig.StreamSettings)
	mss, err := internet.ToMemoryStreamConfig(receiverConfig.StreamSettings)
	if err != nil {
		return nil, newError("failed to parse stream config").Base(err).AtWarning()
	}

	// nil
	if receiverConfig.ReceiveOriginalDestination {
		if mss.SocketSettings == nil {
			mss.SocketSettings = &internet.SocketConfig{}
		}
		if mss.SocketSettings.Tproxy == internet.SocketConfig_Off {
			mss.SocketSettings.Tproxy = internet.SocketConfig_Redirect
		}
		mss.SocketSettings.ReceiveOriginalDestAddress = true
	}

	// From: 10086, To: 10086
	for port := pr.From; port <= pr.To; port++ {
		if net.HasNetwork(nl, net.Network_TCP) {
			// 不知道打到什么地方去了
			newError("creating stream worker on ", address, ":", port).AtDebug().WriteToLog()

			// 0.0.0.0
			fmt.Println(" tcp address:", address)
			worker := &tcpWorker{
				address:         address,
				port:            net.Port(port), // 10086
				proxy:           p,
				stream:          mss,
				recvOrigDest:    receiverConfig.ReceiveOriginalDestination, // 默认值
				tag:             tag, // ''
				dispatcher:      h.mux,
				sniffingConfig:  receiverConfig.GetEffectiveSniffingSettings(), // nil
				uplinkCounter:   uplinkCounter, // 空的
				downlinkCounter: downlinkCounter, // 空的
				ctx:             ctx,
			}
			h.workers = append(h.workers, worker)
		}

		if net.HasNetwork(nl, net.Network_UDP) {
			worker := &udpWorker{
				tag:             tag,
				proxy:           p,
				address:         address,
				port:            net.Port(port),
				dispatcher:      h.mux,
				uplinkCounter:   uplinkCounter,
				downlinkCounter: downlinkCounter,
				stream:          mss,
			}
			h.workers = append(h.workers, worker)
		}
	}


	/**
	
		此时的h
		h := &AlwaysOnInboundHandler{
			proxy:  &Handler{ 在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/inbound.go 
				policyManager: 指向server里features里的同类,
				inboundHandlerManager:  指向server里features里的同类,
				clients: &TimedUserValidator{
					users: [
						&user{
							user: *mUser,
							lastSec: protocol.Timestamp(nowSec - cacheDurationSec),
						},
					],
					userHash: make(map[[16]byte]indexTimePair, 1024), 有值
					hasher:  一个func: protocol.DefaultIDHash,
					baseTime: protocol.Timestamp(time.Now().Unix() - cacheDurationSec*2),
					aeadDecoderHolder: aead.NewAuthIDDecoderHolder(), 有值
				},

				detours: nil,
				usersByEmail: &userByEmail{
					cache:           make(map[string]*protocol.MemoryUser),
					defaultLevel:    0,
					defaultAlterIDs: 32,
				},
				sessionHistory: encoding.NewSessionHistory(),
				secure:          false
			},

			mux: &Server{
				dispatcher: 指向 features 里的 *routing.Dispatcher 实例
			},

			tag:   '',
			woerks: [

				&tcpWorker{
					address: 0.0.0.0,
					port: net.Port(port), // 10086
					proxy: 指向外层的 proxy,

					stream: &MemoryStreamConfig{
						ProtocolName: 'tcp',
						ProtocolSettings: &Config struct { new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
						HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
						AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
					},

					recvOrigDest: false,
					tag: '',
					dispatcher: 指向外层的mux,
					sniffingConfig: nil
					uplinkCounter:   空的接口,
					downlinkCounter: 空的接口,
					ctx: ctx,
				},

			]
		}

		protocol为http时:
		h := &AlwaysOnInboundHandler{
			proxy: &Server{ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/server.go
				config: 空结构体,
				policyManager: 指向features的*policy.Manager
			},

			mux: &Server{
				dispatcher: 指向 features 里的 *routing.Dispatcher 实例
			},

			tag:   '',
			woerks: [

				&tcpWorker{
					address: 0.0.0.0,
					port: net.Port(port), // 10086
					proxy: 指向外层的 proxy,

					stream: &MemoryStreamConfig{
						ProtocolName: 'tcp',
						ProtocolSettings: &Config struct { new了一个空的Config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.pb.go
						HeaderSettings      *serial.TypedMessage `protobuf:"bytes,2,opt,name=header_settings,json=headerSettings,proto3" json:"header_settings,omitempty"`
						AcceptProxyProtocol bool                 `protobuf:"varint,3,opt,name=accept_proxy_protocol,json=acceptProxyProtocol,proto3" json:"accept_proxy_protocol,omitempty"`
					},

					recvOrigDest: false,
					tag: '',
					dispatcher: 指向外层的mux,
					sniffingConfig: nil
					uplinkCounter:   空的接口,
					downlinkCounter: 空的接口,
					ctx: ctx,
				},

			]
		}

	*/

	return h, nil
}

// Start implements common.Runnable.
func (h *AlwaysOnInboundHandler) Start() error {
	for _, worker := range h.workers { // tcpWorker 在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/worker.go
		if err := worker.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Close implements common.Closable.
func (h *AlwaysOnInboundHandler) Close() error {
	var errs []error
	for _, worker := range h.workers {
		errs = append(errs, worker.Close())
	}
	errs = append(errs, h.mux.Close())
	if err := errors.Combine(errs...); err != nil {
		return newError("failed to close all resources").Base(err)
	}
	return nil
}

func (h *AlwaysOnInboundHandler) GetRandomInboundProxy() (interface{}, net.Port, int) {
	if len(h.workers) == 0 {
		return nil, 0, 0
	}
	w := h.workers[dice.Roll(len(h.workers))]
	return w.Proxy(), w.Port(), 9999
}

func (h *AlwaysOnInboundHandler) Tag() string {
	return h.tag
}

func (h *AlwaysOnInboundHandler) GetInbound() proxy.Inbound {
	return h.proxy
}
