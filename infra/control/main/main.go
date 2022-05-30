package main

// cd $(go env GOPATH)/src/v2ray.com/core/infra/control/main
// env CGO_ENABLED=0 go build -o $(go env GOPATH)/src/v2ray.com/core/main/v2ctl -tags confonly -ldflags "-s -w"

import (
	"flag"
	"fmt"
	"os"

	commlog "v2ray.com/core/common/log"
	// _ "v2ray.com/core/infra/conf/command"
	"v2ray.com/core/infra/control"
)

func getCommandName() string {
	if len(os.Args) > 1 {
		return os.Args[1] // config
	}
	return ""
}

func main() {
	// let the v2ctl prints log at stderr
	commlog.RegisterHandler(commlog.NewLogger(commlog.CreateStderrLogWriter()))
	// config
	name := getCommandName()
	fmt.Fprintln(os.Stderr, "commandName:", name)

	
	// ConfigCommand: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/control/config.go
	cmd := control.GetCommand(name)
	fmt.Fprintln(os.Stderr, "++cmd:", cmd)

	if cmd == nil {
		fmt.Fprintln(os.Stderr, "Unknown command:", name)
		fmt.Fprintln(os.Stderr)

		fmt.Println("v2ctl <command>")
		fmt.Println("Available commands:")
		control.PrintUsage()
		return
	}

	if err := cmd.Execute(os.Args[2:]); err != nil {
		hasError := false
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err.Error())
			fmt.Fprintln(os.Stderr)
			hasError = true
		}

			/** 此时的 pbConfig /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto proto结构体
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

		for _, line := range cmd.Description().Usage {
			fmt.Println(line)
		}

		if hasError {
			os.Exit(-1)
		}
	}
}
