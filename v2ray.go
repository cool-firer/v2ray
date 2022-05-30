// +build !confonly

package core

import (
	"context"
	"reflect"
	"sync"
	"fmt"
	"v2ray.com/core/common"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/features"
	"v2ray.com/core/features/dns"
	"v2ray.com/core/features/dns/localdns"
	"v2ray.com/core/features/inbound"
	"v2ray.com/core/features/outbound"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/features/routing"
	"v2ray.com/core/features/stats"
)

// Server is an instance of V2Ray. At any time, there must be at most one Server instance running.
type Server interface {
	common.Runnable
}

// ServerType returns the type of the server.
func ServerType() interface{} {
	return (*Instance)(nil)
}

type resolution struct {
	deps     []reflect.Type
	callback interface{}
}

func getFeature(allFeatures []features.Feature, t reflect.Type) features.Feature {
	fmt.Println("      in getFeature")
	for _, f := range allFeatures {
		fmt.Println("        t:", t, " f.Type():", f.Type(), " reflect.TypeOf(f.Type()):", reflect.TypeOf(f.Type()))
		if reflect.TypeOf(f.Type()) == t {
			return f
		}
	}
	return nil
}

func (r *resolution) resolve(allFeatures []features.Feature) (bool, error) {
	var fs []features.Feature

	fmt.Println("  in resolve")
	for _, d := range r.deps { // 4个参数

		// 当前是dispatcher.Config, 此时allFeatures里只有: v2ray.core.app.log.Config 的 Type()方法, 返回的是nil
		f := getFeature(allFeatures, d) 

		fmt.Println("    d:", d, " f:", f)
		if f == nil {
			return false, nil
		}
		fs = append(fs, f)
	}

	callback := reflect.ValueOf(r.callback)
	var input []reflect.Value

	callbackType := callback.Type()
	for i := 0; i < callbackType.NumIn(); i++ {
		pt := callbackType.In(i)
		for _, f := range fs {
			if reflect.TypeOf(f).AssignableTo(pt) {
				input = append(input, reflect.ValueOf(f))
				break
			}
		}
	}

	if len(input) != callbackType.NumIn() {
		panic("Can't get all input parameters")
	}

	var err error

	// 调用callback, 传入四个features
	ret := callback.Call(input)

	// ret: [<error Value>]
	fmt.Println("ret:", ret, " err:", err)

	errInterface := reflect.TypeOf((*error)(nil)).Elem()
	for i := len(ret) - 1; i >= 0; i-- {
		if ret[i].Type() == errInterface {
			v := ret[i].Interface()
			if v != nil {
				err = v.(error)
			}
			break
		}
	}

	return true, err
}

// Instance combines all functionalities in V2Ray.
type Instance struct {
	access             sync.Mutex
	features           []features.Feature
	featureResolutions []resolution
	running            bool

	ctx context.Context
}

/**
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
	},

	inbound protocol: http
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
				type: 'v2ray.core.proxy.http.ServerConfig',
				value: 空的值
			}
	},
	


*/
func AddInboundHandler(server *Instance, config *InboundHandlerConfig) error {

	// 从features里找到这个类型的
	/**
		type Manager struct {
				access          sync.RWMutex
				untaggedHandler []inbound.Handler
				taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
				running         bool
		}
	*/
	inboundManager := server.GetFeature(inbound.ManagerType()).(inbound.Manager)


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
			workers: [

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


	rawHandler, err := CreateObject(server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(inbound.Handler)
	if !ok {
		return newError("not an InboundHandler")
	}

	// 加到 inboundManager里的 untaggedHandler 里 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/inbound/inbound.go
	if err := inboundManager.AddHandler(server.ctx, handler); err != nil {
		return err
	}
	return nil
}

/**
	configs(Inbound):
	[

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

	],

*/

func addInboundHandlers(server *Instance, configs []*InboundHandlerConfig) error {

	fmt.Print("addInboundHandlers\n")

	for _, inboundConfig := range configs {
		fmt.Println(" ==AddInboundHandler==")
		if err := AddInboundHandler(server, inboundConfig); err != nil {
			return err
		}
		fmt.Println(" ==AddInboundHandler done==")
	}
	fmt.Print("addInboundHandlers Done\n\n")

	return nil
}

func AddOutboundHandler(server *Instance, config *OutboundHandlerConfig) error {
	outboundManager := server.GetFeature(outbound.ManagerType()).(outbound.Manager)

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
	rawHandler, err := CreateObject(server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(outbound.Handler)
	if !ok {
		return newError("not an OutboundHandler")
	}

	// 方法实现在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/outbound.go
	// untaggedHandlers 添加
	// defaultHandler  = handler
	if err := outboundManager.AddHandler(server.ctx, handler); err != nil {
		return err
	}
	return nil
}

/**
		configs: [
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

		]

*/
func addOutboundHandlers(server *Instance, configs []*OutboundHandlerConfig) error {
	fmt.Print("\naddOutboundHandlers\n")
	for _, outboundConfig := range configs {
		if err := AddOutboundHandler(server, outboundConfig); err != nil {
			return err
		}
	}
	fmt.Print("addOutboundHandlers Done\n\n")
	return nil
}

// RequireFeatures is a helper function to require features from Instance in context.
// See Instance.RequireFeatures for more information.
func RequireFeatures(ctx context.Context, callback interface{}) error {
	v := MustFromContext(ctx) // 拿到 ctx里传的值: server实例

	return v.RequireFeatures(callback)
}

// New returns a new V2Ray instance based on given configuration.
// The instance is not started at this point.
// To ensure V2Ray instance works properly, the config must contain one Dispatcher, one InboundHandlerManager and one OutboundHandlerManager. Other features are optional.

	/** config /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto proto结构体
	{
		App: [
			{     
				type: 'v2ray.core.app.log.Config',    
				value: log.Config {  AccessLogType: 0   ErrorLogType: 1    ErrorLogLevel: 2 } 编码成[]byte  
			},  
			{     
				type: 'v2ray.core.app.dispatcher.Config',    
				value: proto.Marshal编码成[]byte  
			},  
			{     
				type: 'v2ray.core.app.proxyman.InboundConfig',    
				value:   
			},  
			{     
				type: 'v2ray.core.app.proxyman.OutboundConfig',    
				value:   
			},
		],

		Inbound: [
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

		],

		Outbound: [
			
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
		]
	}
	**/
func New(config *Config) (*Instance, error) {
	/**
	server: 
	type Instance struct {
		access             sync.Mutex
		features           []features.Feature
		featureResolutions []resolution
		running            bool
		ctx context.Context
	}
	*/
	var server = &Instance{ctx: context.Background()}

	err, done := initInstanceWithConfig(config, server)
	if done {
		return nil, err
	}

	return server, nil
}

func NewWithContext(config *Config, ctx context.Context) (*Instance, error) {
	var server = &Instance{ctx: ctx}

	err, done := initInstanceWithConfig(config, server)
	if done {
		return nil, err
	}

	return server, nil
}


/**
server: 只有ctx有值
	type Instance struct {
		access             sync.Mutex
		features           []features.Feature
		featureResolutions []resolution
		running            bool
		ctx context.Context
	}
config /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto proto结构体
	{
		App: [

			&TypedMessage { 
				Type: 'v2ray.core.app.log.Config',
				value: &log.Config {  value的值被marshal成了[]byte  proto结构体
					AccessLogType: None(0)
					ErrorLogType: Console(1)
					ErrorLogLevel: Warning(2)
				}
			},

			{
				type: 'v2ray.core.app.dispatcher.Config',    
				value: &dispatcher.Config{} proto.Marshal编码成[]byte  
			},

			{
				type: 'v2ray.core.app.proxyman.InboundConfig',    
				value: &proxyman.InboundConfig{}
			},

			{
				type: 'v2ray.core.app.proxyman.OutboundConfig',    
				value: &proxyman.OutboundConfig{}
			},
		],

		Inbound: [

			&core.InboundHandlerConfig{ proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto
				Tag: '',
				ReceiverSettings: &TypedMessage{
					Type: "v2ray.core.app.proxyman.ReceiverConfig",
					Value: &proxyman.ReceiverConfig{
						PortRange: &net.PortRange{ From: 10086, To: 10086 }
					}
				},
				ProxySettings: &TypedMessage{
					Type: 'v2ray.core.proxy.vmess.inbound.Config',
					Value: &inbound.Config{
						SecureEncryptionOnly: false,
						User: [

							protocol.User{
								Account: &TypedMessage:{
									Type: 'v2ray.core.proxy.vemss.Account',
									Value: &vmess.Account{
										Id: "b831381d-6324-4d53-ad4f-8cda48b30811",
										AlterId: 0,
										SecuritySettings: &protocol.SecurityConfig{ Type: AUTO }
									}
								}
							}

						]
					}
				}
			}

		],

		Outbound: [
			&core.OutboundHandlerConfig{  /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto 生成的结构体
				Tag: '',
				SenderSettings: &TypedMessage{
					Type: 'v2ray.core.app.proxyman.SenderConfig',
					Value: &proxyman.SenderConfig{}
				}
				ProxySettings: &TypedMessage{
					Type: 'v2ray.core.proxy.freedom.Config',
					Value: &freedom.Config{
						DomainStrategy: freedom.Config_AS_IS 枚举值,
						UserLevel: 0,
					}
				}
			}
		]
	}

*/
func initInstanceWithConfig(config *Config, server *Instance) (error, bool) {

	// nil
	if config.Transport != nil {
		features.PrintDeprecatedFeatureWarning("global transport settings")
	}
	
	// nil
	if err := config.Transport.Apply(); err != nil {
		return err, true
	}

	for _, appSettings := range config.App {

		// 解码 已经没有了二进制 的 proto 结构体对象

		// 去壳TypedMessage, 拿到Value对应的 proto结构体实例, 用 proto.Message 接收
		settings, err := appSettings.GetInstance()
		if err != nil {
			return err, true
		}
		// Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
		// Type: v2ray.core.app.dispatcher.Config, settings: 
		// Type: v2ray.core.app.proxyman.InboundConfig, settings: 
		// Type: v2ray.core.app.proxyman.OutboundConfig, settings: 
		fmt.Printf("\nType: %s, settings: %+v\n", appSettings.Type, settings)

		/**
			Instance在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/log/log.go 返回一个g, 即obj
			Type: v2ray.core.app.log.Config, settings: error_log_type:Console error_log_level:Warning
			g := &Instance {
				config: config proto结构体,
				active: true,
				accessLogger: nil,
				errorLogger: &generalLogger {
					creator: logWriterCreator 一个func
					buffer:  make(chan Message, 16),
					access:  semaphore.New(1),
					done:    done.New(),
				}
			}

			Type: v2ray.core.app.dispatcher.Config, settings空
			会添加内容到server属性里
			返回的obj: type DefaultDispatcher struct {
				ohm    outbound.Manager
				router routing.Router
				policy policy.Manager
				stats  stats.Manager
			}
			此时的 server
			type Instance struct {
				access    sync.Mutex
				features  []features.Feature: [
					v2ray.core.app.log.Config的g
				]
				featureResolutions []resolution : [

					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}

				]
				running   bool
				ctx context.Context : 有值
			}

			Type: v2ray.core.app.proxyman.InboundConfig settings: 空
	  	返回 m := &Manager{ 即obj
						taggedHandlers: make(map[string]inbound.Handler),
					}
			此时的 server
			type Instance struct {
				access    sync.Mutex
				features  []features.Feature: [

					v2ray.core.app.log.Config的g

					v2ray.core.app.dispatcher.Config的 空结构体
						type DefaultDispatcher struct {
							ohm    outbound.Manager
							router routing.Router
							policy policy.Manager
							stats  stats.Manager
					}
				]

				featureResolutions []resolution : [

					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}

				]
				running   bool
				ctx context.Context : 有值
			}

			Type: v2ray.core.app.proxyman.OutboundConfig settings: 空
	  	返回 m := &Manager{ 即obj
						taggedHandler: make(map[string]outbound.Handler),
					}
			此时的 server
			type Instance struct {
				access    sync.Mutex
				features  []features.Feature: [

					v2ray.core.app.log.Config的g

					v2ray.core.app.dispatcher.Config的 空结构体
						type DefaultDispatcher struct {
							ohm    outbound.Manager
							router routing.Router
							policy policy.Manager
							stats  stats.Manager
					}

					v2ray.core.app.proxyman.InboundConfig 的m
						type Manager struct {
							access          sync.RWMutex
							untaggedHandler []inbound.Handler
							taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
							running         bool
						}
					
				
				]

				featureResolutions []resolution : [

					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}

				]
				running   bool
				ctx context.Context : 有值
			}

		*/
		obj, err := CreateObject(server, settings)
		if err != nil {
			return err, true
		}

		// Type()、Start()、Close() 方法

		// v2ray.core.app.log.Config 可以转
		// v2ray.core.app.dispatcher.Config 可以转
		// v2ray.core.app.proxyman.InboundConfig 可以转
		// v2ray.core.app.proxyman.OutboundConfig 可以转
		if feature, ok := obj.(features.Feature); ok {

			// 把 g 加入到 server.features里
			if err := server.AddFeature(feature); err != nil {
				return err, true
			}
		}
		
	}

	fmt.Print("loop end config.App\n\n")

	/**
		跑完for s:
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
						ohm    outbound.Manager
						router routing.Router
						policy policy.Manager
						stats  stats.Manager
					}

					&Manager { 	v2ray.core.app.proxyman.InboundConfig 的m
						access          sync.RWMutex
						untaggedHandler []inbound.Handler
						taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
						running         bool
					}

					&Manager {	v2ray.core.app.proxyman.OutboundConfig 的m
						access           sync.RWMutex
						defaultHandler   outbound.Handler
						taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
						untaggedHandlers []outbound.Handler
						running          bool
					}
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

	essentialFeatures := []struct {
		Type     interface{}
		Instance features.Feature
	}{
		// { *dns.Client, &Client{} }
		{dns.ClientType(), localdns.New()},

		// { *policy.Manager,  }
		{policy.ManagerType(), policy.DefaultManager{}},

		// { *routing.Router, }
		{routing.RouterType(), routing.DefaultRouter{}},

		// { *status.Manager,  }
		{stats.ManagerType(), stats.NoopManager{}},
	}

	for _, f := range essentialFeatures {
		fmt.Println("f:", f, " f.Type:", f.Type, " reflect.TypeOf(featureType):", reflect.TypeOf(f.Type))
		if server.GetFeature(f.Type) == nil {
			// 全部加进了feature数组
			fmt.Printf("essentialFeatures addFeature, f:%+v \n", f)
			if err := server.AddFeature(f.Instance); err != nil {
				return err, true
			}
		}
		fmt.Print("\n\n")
	}

		/**
		跑完for essentialFeatures:
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

					&Manager { 	v2ray.core.app.proxyman.InboundConfig 的m
						access          sync.RWMutex
						untaggedHandler []inbound.Handler
						taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
						running         bool
					}

					&Manager {	v2ray.core.app.proxyman.OutboundConfig 的m
						access           sync.RWMutex
						defaultHandler   outbound.Handler
						taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
						untaggedHandlers []outbound.Handler
						running          bool
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

	if server.featureResolutions != nil {
		return newError("not all dependency are resolved."), true
	}

	if err := addInboundHandlers(server, config.Inbound); err != nil {
		return err, true
	}

	/** addInboundHandlers 完后, server
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

					&Manager { 	v2ray.core.app.proxyman.InboundConfig 的m
						access          sync.RWMutex
						untaggedHandler []inbound.Handler = [

							&AlwaysOnInboundHandler {
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

								mux: &Server {
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
						running         bool
					}

					&Manager {	v2ray.core.app.proxyman.OutboundConfig 的m
						access           sync.RWMutex
						defaultHandler   outbound.Handler
						taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
						untaggedHandlers []outbound.Handler
						running          bool
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


	if err := addOutboundHandlers(server, config.Outbound); err != nil {
		return err, true
	}

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

					&Manager { 	v2ray.core.app.proxyman.InboundConfig 的m
						access          sync.RWMutex
						untaggedHandler []inbound.Handler = [

							&AlwaysOnInboundHandler {
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
						
						],
						taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
						running         bool
					}

					&Manager {	v2ray.core.app.proxyman.OutboundConfig 的m
						access           sync.RWMutex
						defaultHandler   outbound.Handler = 指向untaggedHandlers[0],
						taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
						untaggedHandlers []outbound.Handler = [

							&Handler{
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
						running          bool
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
	return nil, false
}

// Type implements common.HasType.
func (s *Instance) Type() interface{} {
	return ServerType()
}

// Close shutdown the V2Ray instance.
func (s *Instance) Close() error {
	s.access.Lock()
	defer s.access.Unlock()

	s.running = false

	var errors []interface{}
	for _, f := range s.features {
		if err := f.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return newError("failed to close all features").Base(newError(serial.Concat(errors...)))
	}

	return nil
}

// RequireFeatures registers a callback, which will be called when all dependent features are registered.
// The callback must be a func(). All its parameters must be features.Feature.
func (s *Instance) RequireFeatures(callback interface{}) error {
	callbackType := reflect.TypeOf(callback) // 接收到的动态值
	if callbackType.Kind() != reflect.Func {
		panic("not a function")
	}

	/**
		s是server实例
		在处理 v2ray.core.app.dispatcher.Config 时, 
		callback: 

			func(om outbound.Manager, router routing.Router, pm policy.Manager, sm stats.Manager) error {
				return d.Init(config.(*Config), om, router, pm, sm)
			})

		freedom.Config, callback:
		func(pm policy.Manager, d dns.Client) error {
			return h.Init(config.(*Config), pm, d)
		})
	*/

	var featureTypes []reflect.Type
	for i := 0; i < callbackType.NumIn(); i++ { // 4个入参

		// v: *outbound.Manager  Kind(): ptr
		// v: *routing.Router  Kind(): ptr
		// v: *policy.Manager  Kind(): ptr
		//  v: *stats.Manager  Kind(): ptr
		fmt.Println("  i:", i, " v:", reflect.PtrTo(callbackType.In(i)), " Kind():", reflect.PtrTo(callbackType.In(i)).Kind())
		featureTypes = append(featureTypes, reflect.PtrTo(callbackType.In(i)))
	}

	r := resolution{
		deps:     featureTypes, // callback函数的四个参数
		callback: callback,	// callback函数本身
	}

	// 此时的s.features只有 v2ray.core.app.log.Config 对应的 Instance g
	// 返回 failse, nil 了
	if finished, err := r.resolve(s.features); finished {
		return err
	}

	/**
		此时的 server, 即s
		server: 只有ctx有值
			type Instance struct {
				access    sync.Mutex

				features  []features.Feature: [
					g
				]

				featureResolutions []resolution : [
					r封装了 v2ray.core.app.dispatcher.Config的callback
				]

				running            bool
				ctx context.Context : 有值
		}

			freedom.Config, callback, 此时的server:
				{
					featureResolutions []resolution : [

						r封装了 v2ray.core.app.dispatcher.Config的callback,

						r封装了/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go 的callback,
					]
				}

	*/
	s.featureResolutions = append(s.featureResolutions, r)
	return nil
}

// AddFeature registers a feature into current Instance.
func (s *Instance) AddFeature(feature features.Feature) error {
	fmt.Println("  in AddFeature")
	s.features = append(s.features, feature)

	if s.running {
		if err := feature.Start(); err != nil {
			newError("failed to start feature").Base(err).WriteToLog()
		}
		return nil
	}

	/**
	  处理 v2ray.core.app.log.Config 时
		此时的 server, 即s
		server: 只有ctx有值
			type Instance struct {
				access    sync.Mutex
				features  []features.Feature: [
					v2ray.core.app.log.Config的g
				]
				featureResolutions []resolution
				running            bool
				ctx context.Context : 有值
		}

		处理 v2ray.core.app.dispatcher.Config 时
		此时的 server, 即s
		server: 
			type Instance struct {
				access    sync.Mutex

				features  []features.Feature: [
					v2ray.core.app.log.Config的g

					v2ray.core.app.dispatcher.Config的 空结构体
						type DefaultDispatcher struct {
							ohm    outbound.Manager
							router routing.Router
							policy policy.Manager
							stats  stats.Manager
					}

				]

				featureResolutions []resolution : [
					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}
				]

				running            bool
				ctx context.Context : 有值
		}

		处理 v2ray.core.app.proxyman.InboundConfig 时
		此时的 server, 即s
		server:
			type Instance struct {
				access    sync.Mutex

				features  []features.Feature: [
					v2ray.core.app.log.Config的g

					v2ray.core.app.dispatcher.Config的 空结构体
						type DefaultDispatcher struct {
							ohm    outbound.Manager
							router routing.Router
							policy policy.Manager
							stats  stats.Manager
					}

					v2ray.core.app.proxyman.InboundConfig 的m
						type Manager struct {
							access          sync.RWMutex
							untaggedHandler []inbound.Handler
							taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
							running         bool
						}

				]

				featureResolutions []resolution : [
					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}
				]

				running            bool
				ctx context.Context : 有值
		}

		处理 v2ray.core.app.proxyman.OutboundConfig 时
		此时的 server, 即s
		server:
			type Instance struct {
				access    sync.Mutex

				features  []features.Feature: [
					v2ray.core.app.log.Config的g

					v2ray.core.app.dispatcher.Config的 空结构体
						type DefaultDispatcher struct {
							ohm    outbound.Manager
							router routing.Router
							policy policy.Manager
							stats  stats.Manager
					}

					v2ray.core.app.proxyman.InboundConfig 的m
					type Manager struct {
							access          sync.RWMutex
							untaggedHandler []inbound.Handler
							taggedHandlers  map[string]inbound.Handler ==> make(map[string]inbound.Handler),
							running         bool
					}

					v2ray.core.app.proxyman.OutboundConfig 的m
					type Manager struct {
							access           sync.RWMutex
							defaultHandler   outbound.Handler
							taggedHandler    map[string]outbound.Handler ==> make(map[string]outbound.Handler),
							untaggedHandlers []outbound.Handler
							running          bool
					}

				]

				featureResolutions []resolution : [
					r封装了 v2ray.core.app.dispatcher.Config的callback
					r := resolution{
						deps:     featureTypes, // callback函数的四个参数
						callback: callback,	// callback函数本身
					}
				]

				running            bool
				ctx context.Context : 有值
		}

	*/

	// 切片默认是nil
	if s.featureResolutions == nil {
		fmt.Print(" s.featureResolutions is nil return\n\n")
		return nil
	}

	// 这里没看懂
	var pendingResolutions []resolution
	for _, r := range s.featureResolutions {
		finished, err := r.resolve(s.features) //此时s.features有两个: log、dispatcher
		if finished && err != nil {
			return err
		}
		if !finished {
			pendingResolutions = append(pendingResolutions, r)
		}
	}


	if len(pendingResolutions) == 0 {
		s.featureResolutions = nil
	} else if len(pendingResolutions) < len(s.featureResolutions) {
		s.featureResolutions = pendingResolutions
	}

	fmt.Println("  end AddFeature")
	return nil
}

// GetFeature returns a feature of the given type, or nil if such feature is not registered.
func (s *Instance) GetFeature(featureType interface{}) features.Feature {
	return getFeature(s.features, reflect.TypeOf(featureType))
}

// Start starts the V2Ray instance, including all registered features. When Start returns error, the state of the instance is unknown.
// A V2Ray instance can be started only once. Upon closing, the instance is not guaranteed to start again.
//
// v2ray:api:stable
func (s *Instance) Start() error {
	s.access.Lock()
	defer s.access.Unlock()
	s.running = true

	for _, f := range s.features {

		/**
			v2ray.core.app.proxyman.InboundConfig:
				设置m.running = true
				遍历 untaggedHandler, 调用Start()
		*/
		fmt.Print("feature:", f, " reflect.TypeOf(f.Type()):", reflect.TypeOf(f.Type()), " start...\n")
		if err := f.Start(); err != nil {
			return err
		}
		fmt.Print("start done\n\n")
	}

	newError("V2Ray ", Version(), " started").AtWarning().WriteToLog()

	return nil
}
