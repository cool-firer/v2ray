// +build !confonly

package core

import (
	"io"
	"strings"

	"github.com/golang/protobuf/proto"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/cmdarg"
	"v2ray.com/core/main/confloader"
)

// ConfigFormat is a configurable format of V2Ray config file.
type ConfigFormat struct {
	Name      string
	Extension []string
	Loader    ConfigLoader
}

// ConfigLoader is a utility to load V2Ray config from external source.
type ConfigLoader func(input interface{}) (*Config, error)

var (
	configLoaderByName = make(map[string]*ConfigFormat)
	configLoaderByExt  = make(map[string]*ConfigFormat)
)

// RegisterConfigLoader add a new ConfigLoader.
func RegisterConfigLoader(format *ConfigFormat) error {
	name := strings.ToLower(format.Name)
	if _, found := configLoaderByName[name]; found {
		return newError(format.Name, " already registered.")
	}
	configLoaderByName[name] = format

	for _, ext := range format.Extension {
		lext := strings.ToLower(ext)
		if f, found := configLoaderByExt[lext]; found {
			return newError(ext, " already registered to ", f.Name)
		}
		configLoaderByExt[lext] = format
	}

	return nil
}

func getExtension(filename string) string {
	idx := strings.LastIndexByte(filename, '.')
	if idx == -1 {
		return ""
	}
	return filename[idx+1:]
}

// LoadConfig loads config with given format from given source.
// input accepts 2 different types:
// * []string slice of multiple filename/url(s) to open to read
// * io.Reader that reads a config content (the original way)

// 'json', './config.json', [ './config.json' ]

// 第二次调用: "protobuf", "", r(io.Reader)
func LoadConfig(formatName string, filename string, input interface{}) (*Config, error) {
	ext := getExtension(filename) // filename仅仅用来提取后缀, 这样传参有点多余了

	// 有扩展名
	if len(ext) > 0 {

		// found
		// configLoaderByExt 的填充pb, json竟然是分开在两个文件里, 不能放一个地方填充么？
		// json的填充: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/json/config_json.go
		if f, found := configLoaderByExt[ext]; found {
			return f.Loader(input) // input: []string切片
		}
	}

	// 第二次, 在这个文件的最下面, protobuf, '', v2ctrl输出流
	// 在这个文件的最下面
	if f, found := configLoaderByName[formatName]; found {
		return f.Loader(input)
	}

	return nil, newError("Unable to load config in ", formatName).AtWarning()
}

/** 此时的 data是从v2ctl里拿到的字节流 pbConfig /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto proto结构体
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
func loadProtobufConfig(data []byte) (*Config, error) {

	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto
	config := new(Config) // new 的作用是为类型申请一片内存空间，并返回指向这片内存的指针
	
	if err := proto.Unmarshal(data, config); err != nil { // 从v2ctl拿到的字节编码, 转成proto结构体
		return nil, err
	}
	
	return config, nil
}

func init() {
	// 没注册Json的, 在 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/json/config_json.go 里注册
	// 在/Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/main.go 里通过引入 _ "v2ray.com/core/main/distro/all"
	common.Must(RegisterConfigLoader(&ConfigFormat{
		Name:      "Protobuf",
		Extension: []string{"pb"},
		Loader: func(input interface{}) (*Config, error) {
			switch v := input.(type) {
			case cmdarg.Arg:
				r, err := confloader.LoadConfig(v[0])
				common.Must(err)
				data, err := buf.ReadAllToBytes(r)
				common.Must(err)
				return loadProtobufConfig(data)
			case io.Reader:
				data, err := buf.ReadAllToBytes(v)
				common.Must(err)
				return loadProtobufConfig(data)
			default:
				return nil, newError("unknow type")
			}
		},
	}))
}
