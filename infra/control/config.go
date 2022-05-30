package control

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/protobuf/proto"
	"v2ray.com/core/common"
	"v2ray.com/core/infra/conf"
	"v2ray.com/core/infra/conf/serial"
)

// ConfigCommand is the json to pb convert struct
type ConfigCommand struct{}

// Name for cmd usage
func (c *ConfigCommand) Name() string {
	return "config"
}

// Description for help usage
func (c *ConfigCommand) Description() Description {
	return Description{
		Short: "merge multiple json config",
		Usage: []string{"v2ctl config config.json c1.json c2.json <url>.json"},
	}
}

// Execute real work here.
// args: [ /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/config.json ]
// 读取config.json内容 -> 转成pb类型 -> 转成字节流 -> 打印到标准输出
func (c *ConfigCommand) Execute(args []string) error {
	if len(args) < 1 {
		return newError("empty config list")
	}

	conf := &conf.Config{}
	for _, arg := range args {
		ctllog.Println("Read config: ", arg)

		// LoadArg()读取文件内容, 封装成一个buffer
		// r: io.Reader
		r, err := c.LoadArg(arg)
		common.Must(err)

		// unmarshal解码json字符串, 成结构体c
		c, err := serial.DecodeJSONConfig(r)
			/** c对应 json结构体
 	Config json结构体 {
		Port            uint16                 `json:"port"` // Port of this Point server. Deprecated.
		LogConfig       *LogConfig             `json:"log"`
		RouterConfig    *RouterConfig          `json:"routing"`
		DNSConfig       *DnsConfig             `json:"dns"`

		InboundConfigs  []InboundDetourConfig  `json:"inbounds"`
		OutboundConfigs []OutboundDetourConfig `json:"outbounds"`

		Transport       *TransportConfig       `json:"transport"`
		Policy          *PolicyConfig          `json:"policy"`
		Api             *ApiConfig             `json:"api"`
		Stats           *StatsConfig           `json:"stats"`
		Reverse         *ReverseConfig         `json:"reverse"`
	}
	**/
		if err != nil {
			ctllog.Fatalln(err)
		}
		conf.Override(c, arg)
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
	pbConfig, err := conf.Build()
	if err != nil {
		return err
	}

	// 转为字节编码了
	bytesConfig, err := proto.Marshal(pbConfig)
	if err != nil {
		return newError("failed to marshal proto config").Base(err)
	}

	// 写到输出
	if _, err := os.Stdout.Write(bytesConfig); err != nil {
		return newError("failed to write proto config").Base(err)
	}

	return nil
}

// LoadArg loads one arg, maybe an remote url, or local file path
func (c *ConfigCommand) LoadArg(arg string) (out io.Reader, err error) {

	var data []byte
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		data, err = FetchHTTPContent(arg)
	} else if arg == "stdin:" {
		data, err = ioutil.ReadAll(os.Stdin)
	} else {
		data, err = ioutil.ReadFile(arg)
	}

	if err != nil {
		return
	}
	out = bytes.NewBuffer(data)
	return
}

func init() {
	common.Must(RegisterCommand(&ConfigCommand{}))
}
