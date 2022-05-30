	/** addOutboundHandlers 完后, server
		server:
			type Instance struct {
				access    sync.Mutex

				features  []features.Feature: [

					&Instance{ v2ray.core.app.log.Config的 *log.Instance
						config: 指向pb log.Config结构体对象,
						active: true,
						accessLogger: nil,
						errorLogger: &generalLogger{
							creator: 一个func,
							buffer: make(chan Message, 16),
							access: semephore.New(1),
							done: done.New(),
						}
					}

					&DefaultDispatcher { 	v2ray.core.app.dispatcher.Config的 空结构体
						ohm    outbound.Manager = 指向 features.outbound.Manager
						router routing.Router = 指向 features.routing.Router
						policy policy.Manager = 指向 features.policy.Manager
						stats  stats.Manager = 指向 features.status.Manager
					}

          // 调用Start()方法时
					&Manager { 	v2ray.core.app.proxyman.InboundConfig 的m
						access          sync.RWMutex
						untaggedHandler []inbound.Handler = [

							&AlwaysOnInboundHandler { /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/always.go
								proxy:  &Handler{ 在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/inbound.go 
									policyManager: 指向server里features里的同类,
									inboundHandlerManager:  指向server里features里的同类,
									clients: &TimedUserValidator{
										users: [
											&user{ user: *mUser, lastSec: protocol.Timestamp(nowSec - cacheDurationSec)	},
										],
										userHash: make(map[[16]byte]indexTimePair, 1024), 有值
										hasher:  一个func: protocol.DefaultIDHash,
										baseTime: protocol.Timestamp(time.Now().Unix() - cacheDurationSec*2),
										aeadDecoderHolder: aead.NewAuthIDDecoderHolder(), 有值
									},

									detours: nil,
									usersByEmail: &userByEmail {
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
								workers: [
									&tcpWorker{ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/worker.go
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

                    hub internet.Listener = &Listener{ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/hub.go
			                listener: listener, // 原生的 net.Listener
			                config:   tcpSettings, // 空的结构体
			                addConn:  handler, // 回调
		                },
									},
								]
							},

							protocol为http时:
							&AlwaysOnInboundHandler{
								proxy: &Server {
									/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/server.go
									config: 空结构体,
									policyManager: 指向features的*policy.Manager
								},

								mux: &Server { /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/mux/server.go
									dispatcher: 指向 features 里的 *routing.Dispatcher 实例
								},

								tag:   '',
								woerks: [
									&tcpWorker{
										/Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/worker.go
										address: 0.0.0.0,
										port: net.Port(port), // 1080
										proxy: 指向外层的 proxy,

										stream: &MemoryStreamConfig {
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
							},

						
						],
						taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
						running  bool = true
					}

           // 调用Start()方法时
					&Manager {	v2ray.core.app.proxyman.OutboundConfig 的m
						access           sync.RWMutex
						defaultHandler   outbound.Handler = 指向untaggedHandlers[0],
						taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
						untaggedHandlers []outbound.Handler = [

							&Handler { /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/handler.go
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
							},

						],
						running bool = true
					},

					&localdns.Client{},
					&policy.Manager{},

          
					&routing.Router{},

					&stats.Manager{}
				]

				featureResolutions []resolution : [

					r := resolution{
						// callback函数的四个参数Type
						// func: (om outbound.Manager, router routing.Router, pm policy.Manager, sm stats.Manager)
						deps:     featureTypes,
						callback: dispatcher.Config时的func, 闭包引用 features 里面的 DefaultDispatcher
					}
				]

				running            bool
				ctx context.Context : 有值
		}
	*/