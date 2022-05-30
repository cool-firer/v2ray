package json

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"io"
	"fmt"
	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/cmdarg"
	"v2ray.com/core/infra/conf/serial"
	"v2ray.com/core/main/confloader"
)

func init() {
	common.Must(core.RegisterConfigLoader(&core.ConfigFormat{
		Name:      "JSON",
		Extension: []string{"json"},

		// input:  [ './config.json' ]
		Loader: func(input interface{}) (*core.Config, error) {
			switch v := input.(type) { // 
			case cmdarg.Arg:
				fmt.Printf("++json loader:%+v\n", v) // [ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/config.json ]
				
				/**
					1. 跑v2ctrl命令, 传入config.json
					2. proto结构体, 转成字节流, 输出到stdout, 拿到输出, 转成io.Reader
				*/
				// r: io.Reader
				r, err := confloader.LoadExtConfig(v) // 
				if err != nil {
					return nil, newError("failed to execute v2ctl to convert config file.").Base(err).AtWarning()
				}
				return core.LoadConfig("protobuf", "", r)

			case io.Reader:
				return serial.LoadJSONConfig(v)
			default:
				return nil, newError("unknow type")
			}
		},
	}))
}

/** 此时的 pbConfig /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto proto结构体
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
**/
