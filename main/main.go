package main

//go:generate go run v2ray.com/core/common/errors/errorgen

// env CGO_ENABLED=0 go build -o ./v2ray -ldflags "-s -w"
//  ./v2ray --config=./config.json

// cd $(go env GOPATH)/src/v2ray.com/core/infra/control/main
// env CGO_ENABLED=0 go build -o $HOME/v2ctl -tags confonly -ldflags "-s -w"


/**
	不优化build: env CGO_ENABLED=0 go build -gcflags="all=-N -l" -o ./v2ray
	调试:	dlv exec ./v2ray -- -config ./config.json
*/

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"v2ray.com/core"
	"v2ray.com/core/common/cmdarg"
	"v2ray.com/core/common/platform"
	_ "v2ray.com/core/main/distro/all"
)

var (
	configFiles cmdarg.Arg // "Config file for V2Ray.", the option is customed type, parse in main
	configDir   string
	version     = flag.Bool("version", false, "Show current version of V2Ray.")
	test        = flag.Bool("test", false, "Test config file only, without launching V2Ray server.")
	format      = flag.String("format", "json", "Format of input file.")

	/* We have to do this here because Golang's Test will also need to parse flag, before
	 * main func in this file is run.
	 */
	_ = func() error {

		flag.Var(&configFiles, "config", "Config file for V2Ray. Multiple assign is accepted (only json). Latter ones overrides the former ones.")
		flag.Var(&configFiles, "c", "Short alias of -config")
		flag.StringVar(&configDir, "confdir", "", "A dir with multiple json config")

		return nil
	}()
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func dirExists(file string) bool {
	if file == "" {
		return false
	}
	info, err := os.Stat(file)
	return err == nil && info.IsDir()
}

func readConfDir(dirPath string) {
	confs, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatalln(err)
	}
	for _, f := range confs {
		if strings.HasSuffix(f.Name(), ".json") {
			configFiles.Set(path.Join(dirPath, f.Name()))
		}
	}
}

func getConfigFilePath() (cmdarg.Arg, error) {
	if dirExists(configDir) {
		log.Println("Using confdir from arg:", configDir)
		readConfDir(configDir)
	} else {
		if envConfDir := platform.GetConfDirPath(); dirExists(envConfDir) {
			log.Println("Using confdir from env:", envConfDir)
			readConfDir(envConfDir)
		}
	}

	if len(configFiles) > 0 {
		return configFiles, nil
	}

	if workingDir, err := os.Getwd(); err == nil {
		configFile := filepath.Join(workingDir, "config.json")
		if fileExists(configFile) {
			log.Println("Using default config: ", configFile)
			return cmdarg.Arg{configFile}, nil
		}
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		log.Println("Using config from env: ", configFile)
		return cmdarg.Arg{configFile}, nil
	}

	log.Println("Using config from STDIN")
	return cmdarg.Arg{"stdin:"}, nil
}

func GetConfigFormat() string {
	switch strings.ToLower(*format) { // 默认json
	case "pb", "protobuf":
		return "protobuf"
	default:
		return "json"
	}
}

func startV2Ray() (core.Server, error) {

	// configFiles string数组 配置文件路径 [ ./config.json ]
	configFiles, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	// GetConfigFormat(): 默认 'json'
	

	// 调用了v2ctl，读取config.json, 输出到stdout
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

	// GetConfigFormat(): 'json'
	config, err := core.LoadConfig(GetConfigFormat(), configFiles[0], configFiles)
	if err != nil {
		return nil, newError("failed to read config files: [", configFiles.String(), "]").Base(err)
	}

	// fmt.Printf("++config:%+v\n", config)

	server, err := core.New(config)
	if err != nil {
		return nil, newError("failed to create server").Base(err)
	}

	return server, nil
}

func printVersion() {
	version := core.VersionStatement()
	for _, s := range version {
		fmt.Println(s)
	}
}

func main() {

	// v2ray --config=./config.json
	fmt.Println("I am coming")

	flag.Parse()

	printVersion()

	if *version {
		return
	}

	server, err := startV2Ray()
	if err != nil {
		fmt.Println(err)
		// Configuration error. Exit with a special value to prevent systemd from restarting.
		os.Exit(23)
	}

	if *test {
		fmt.Println("Configuration OK.")
		os.Exit(0)
	}

	if err := server.Start(); err != nil {
		fmt.Println("Failed to start", err)
		os.Exit(-1)
	}
	defer server.Close()

	// Explicitly triggering GC to remove garbage from config loading.
	runtime.GC()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
		<-osSignals
	}
}
