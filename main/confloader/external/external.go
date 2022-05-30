package external

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/platform/ctlcmd"
	"v2ray.com/core/main/confloader"
)

func ConfigLoader(arg string) (out io.Reader, err error) {

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

func FetchHTTPContent(target string) ([]byte, error) {

	parsedTarget, err := url.Parse(target)
	if err != nil {
		return nil, newError("invalid URL: ", target).Base(err)
	}

	if s := strings.ToLower(parsedTarget.Scheme); s != "http" && s != "https" {
		return nil, newError("invalid scheme: ", parsedTarget.Scheme)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(&http.Request{
		Method: "GET",
		URL:    parsedTarget,
		Close:  true,
	})
	if err != nil {
		return nil, newError("failed to dial to ", target).Base(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, newError("unexpected HTTP status code: ", resp.StatusCode)
	}

	content, err := buf.ReadAllToBytes(resp.Body)
	if err != nil {
		return nil, newError("failed to read HTTP response").Base(err)
	}

	return content, nil
}

// files: [ ./config.json ]
func ExtConfigLoader(files []string) (io.Reader, error) {

	/**
		ctlcmd.Run():
		1、执行 v2ctl 命令 参数:  [config, /Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/config.json]
		v2ctl config xxx/config.json

		2、拿到标准输出, 放到buffer
	**/

	/**
		v2ctrl命令流程:
			1. 找到 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/control/config.go
			2. 读取传入的config.json、转成pb结构体 => 再json.marshal()转成字节 => 打印到标准输出
		
	**/
	buf, err := ctlcmd.Run(append([]string{"config"}, files...), os.Stdin)
	if err != nil {
		return nil, err
	}

	return strings.NewReader(buf.String()), nil
}

// 执行v2ctl, 此时拿到的输出
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

func init() {
	confloader.EffectiveConfigFileLoader = ConfigLoader
	confloader.EffectiveExtConfigLoader = ExtConfigLoader
}
