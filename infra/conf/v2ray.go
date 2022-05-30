package conf

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"v2ray.com/core"
	"v2ray.com/core/app/dispatcher"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/stats"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/transport/internet/xtls"
)

var (
	inboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{ // map[string][func]
		"dokodemo-door": func() interface{} { return new(DokodemoConfig) },
		"http":          func() interface{} { return new(HttpServerConfig) },
		"shadowsocks":   func() interface{} { return new(ShadowsocksServerConfig) },
		"socks":         func() interface{} { return new(SocksServerConfig) },
		"vless":         func() interface{} { return new(VLessInboundConfig) },

		"vmess":         func() interface{} { return new(VMessInboundConfig) },

		"trojan":        func() interface{} { return new(TrojanServerConfig) },
		"mtproto":       func() interface{} { return new(MTProtoServerConfig) },
	}, "protocol", "settings")

	outboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"blackhole":   func() interface{} { return new(BlackholeConfig) },

		"freedom":     func() interface{} { return new(FreedomConfig) },

		"http":        func() interface{} { return new(HttpClientConfig) },
		"shadowsocks": func() interface{} { return new(ShadowsocksClientConfig) },
		"socks":       func() interface{} { return new(SocksClientConfig) },
		"vless":       func() interface{} { return new(VLessOutboundConfig) },
		"vmess":       func() interface{} { return new(VMessOutboundConfig) },
		"trojan":      func() interface{} { return new(TrojanClientConfig) },
		"mtproto":     func() interface{} { return new(MTProtoClientConfig) },
		"dns":         func() interface{} { return new(DnsOutboundConfig) },
	}, "protocol", "settings")

	ctllog = log.New(os.Stderr, "v2ctl> ", 0)
)

func toProtocolList(s []string) ([]proxyman.KnownProtocols, error) {
	kp := make([]proxyman.KnownProtocols, 0, 8)
	for _, p := range s {
		switch strings.ToLower(p) {
		case "http":
			kp = append(kp, proxyman.KnownProtocols_HTTP)
		case "https", "tls", "ssl":
			kp = append(kp, proxyman.KnownProtocols_TLS)
		default:
			return nil, newError("Unknown protocol: ", p)
		}
	}
	return kp, nil
}

type SniffingConfig struct {
	Enabled      bool        `json:"enabled"`
	DestOverride *StringList `json:"destOverride"`
}

// Build implements Buildable.
func (c *SniffingConfig) Build() (*proxyman.SniffingConfig, error) {
	var p []string
	if c.DestOverride != nil {
		for _, domainOverride := range *c.DestOverride {
			switch strings.ToLower(domainOverride) {
			case "http":
				p = append(p, "http")
			case "tls", "https", "ssl":
				p = append(p, "tls")
			default:
				return nil, newError("unknown protocol: ", domainOverride)
			}
		}
	}

	return &proxyman.SniffingConfig{
		Enabled:             c.Enabled,
		DestinationOverride: p,
	}, nil
}

type MuxConfig struct {
	Enabled     bool  `json:"enabled"`
	Concurrency int16 `json:"concurrency"`
}

// Build creates MultiplexingConfig, Concurrency < 0 completely disables mux.
func (m *MuxConfig) Build() *proxyman.MultiplexingConfig {
	if m.Concurrency < 0 {
		return nil
	}

	var con uint32 = 8
	if m.Concurrency > 0 {
		con = uint32(m.Concurrency)
	}

	return &proxyman.MultiplexingConfig{
		Enabled:     m.Enabled,
		Concurrency: con,
	}
}

type InboundDetourAllocationConfig struct {
	Strategy    string  `json:"strategy"`
	Concurrency *uint32 `json:"concurrency"`
	RefreshMin  *uint32 `json:"refresh"`
}

// Build implements Buildable.
func (c *InboundDetourAllocationConfig) Build() (*proxyman.AllocationStrategy, error) {
	config := new(proxyman.AllocationStrategy)
	switch strings.ToLower(c.Strategy) {
	case "always":
		config.Type = proxyman.AllocationStrategy_Always
	case "random":
		config.Type = proxyman.AllocationStrategy_Random
	case "external":
		config.Type = proxyman.AllocationStrategy_External
	default:
		return nil, newError("unknown allocation strategy: ", c.Strategy)
	}
	if c.Concurrency != nil {
		config.Concurrency = &proxyman.AllocationStrategy_AllocationStrategyConcurrency{
			Value: *c.Concurrency,
		}
	}

	if c.RefreshMin != nil {
		config.Refresh = &proxyman.AllocationStrategy_AllocationStrategyRefresh{
			Value: *c.RefreshMin,
		}
	}

	return config, nil
}

type InboundDetourConfig struct {
	Protocol       string                         `json:"protocol"`
	PortRange      *PortRange                     `json:"port"` // 这个是自定义实现了unmarshal方法
	ListenOn       *Address                       `json:"listen"`
	Settings       *json.RawMessage               `json:"settings"`
	Tag            string                         `json:"tag"`
	Allocation     *InboundDetourAllocationConfig `json:"allocate"`
	StreamSetting  *StreamConfig                  `json:"streamSettings"`
	DomainOverride *StringList                    `json:"domainOverride"`
	SniffingConfig *SniffingConfig                `json:"sniffing"`
}

// Build implements Buildable.
/**
	c是json结构体:
	type InboundDetourConfig struct {
		Protocol       string                         `json:"protocol"`  --> 'vmess'
		PortRange      *PortRange                     `json:"port"` // 这个是自定义实现了unmarshal方法 --> 10086
		Settings       *json.RawMessage               `json:"settings"` : { clients: [ { id: xxx } ] }
	}
**/
func (c *InboundDetourConfig) Build() (*core.InboundHandlerConfig, error) {
	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
	receiverSettings := &proxyman.ReceiverConfig{} // ReceiverConfig proto 结构体

	if c.PortRange == nil {
		return nil, newError("port range not specified in InboundDetour.")
	}
	receiverSettings.PortRange = c.PortRange.Build() // PortRange proto 结构体

	if c.ListenOn != nil {
		if c.ListenOn.Family().IsDomain() {
			return nil, newError("unable to listen on domain address: ", c.ListenOn.Domain())
		}
		receiverSettings.Listen = c.ListenOn.Build()
	}
	if c.Allocation != nil {
		concurrency := -1
		if c.Allocation.Concurrency != nil && c.Allocation.Strategy == "random" {
			concurrency = int(*c.Allocation.Concurrency)
		}
		portRange := int(c.PortRange.To - c.PortRange.From + 1)
		if concurrency >= 0 && concurrency >= portRange {
			return nil, newError("not enough ports. concurrency = ", concurrency, " ports: ", c.PortRange.From, " - ", c.PortRange.To)
		}

		as, err := c.Allocation.Build()
		if err != nil {
			return nil, err
		}
		receiverSettings.AllocationStrategy = as
	}
	if c.StreamSetting != nil {
		ss, err := c.StreamSetting.Build()
		if err != nil {
			return nil, err
		}
		if ss.SecurityType == serial.GetMessageType(&xtls.Config{}) && !strings.EqualFold(c.Protocol, "vless") {
			return nil, newError("XTLS only supports VLESS for now.")
		}
		receiverSettings.StreamSettings = ss
	}
	if c.SniffingConfig != nil {
		s, err := c.SniffingConfig.Build()
		if err != nil {
			return nil, newError("failed to build sniffing config").Base(err)
		}
		receiverSettings.SniffingSettings = s
	}
	if c.DomainOverride != nil {
		kp, err := toProtocolList(*c.DomainOverride)
		if err != nil {
			return nil, newError("failed to parse inbound detour config").Base(err)
		}
		receiverSettings.DomainOverride = kp
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings) // *json.RawMessage强转成字节切片
		// settings内容: { clients: [ { id: xxx }, ] }
	}

	/**
	settings: byte切片 {  
		  clients: [
		   { "id": "b831381d-6324-4d53-ad4f-8cda48b30811" }		 
			] 
		}
	c.Protocol: 'vmess'
	**/
	// rawConfig: VMessInboundConfig结构体: { Users: 对应 [ {id: xxxx} ] 二进制流 }

	/**
		rawConfig: VMessInboundConfig 普通结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/conf/vmess.go
		type VMessInboundConfig struct {
			Users        []json.RawMessage   `json:"clients"` [ {id: xxx}, ] [  二进制流,  ] 用数组包裹的二进制流
			Features     *FeaturesConfig     `json:"features"`
			Defaults     *VMessDefaultConfig `json:"default"`
			DetourConfig *VMessDetourConfig  `json:"detour"`
			SecureOnly   bool                `json:"disableInsecureEncryption"`
		}

		c.Protocol是http时 new一个普通结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/conf/http.go
		rawConfig: type HttpServerConfig struct {
			Timeout     uint32         `json:"timeout"`
			Accounts    []*HttpAccount `json:"accounts"`
			Transparent bool           `json:"allowTransparent"`
			UserLevel   uint32         `json:"userLevel"`
		}
	**/
	rawConfig, err := inboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, newError("failed to load inbound detour config.").Base(err)
	}
	// false
	if dokodemoConfig, ok := rawConfig.(*DokodemoConfig); ok {
		receiverSettings.ReceiveOriginalDestination = dokodemoConfig.Redirect
	}

	/**
	config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/config.proto 生成的结构体
	User: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/protocol/user.proto 生成的结构体

 		ts 最终的config(proto生成的结构体): {
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
	*/
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, err
	}

	/**
		core.InboundHandlerConfig proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/config.proto
		{
			Tag: ''
			ReceiverSettings: {
					type:"v2ray.core.app.proxyman.ReceiverConfig",

					value是 proto结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
					value: { 
						PortRange: net.PortRange proto结构体
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

				对于http来说 value:
				type: 'v2ray.core.proxy.http.Config'
				/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/config.go

			}
		}
	*/

	return &core.InboundHandlerConfig{
		Tag:              c.Tag,
		ReceiverSettings: serial.ToTypedMessage(receiverSettings),
		ProxySettings:    serial.ToTypedMessage(ts),
	}, nil
}

type OutboundDetourConfig struct {
	Protocol      string           `json:"protocol"`
	SendThrough   *Address         `json:"sendThrough"`
	Tag           string           `json:"tag"`
	Settings      *json.RawMessage `json:"settings"`
	StreamSetting *StreamConfig    `json:"streamSettings"`
	ProxySettings *ProxyConfig     `json:"proxySettings"`
	MuxSettings   *MuxConfig       `json:"mux"`
}

// Build implements Buildable.
/**
	c是普通的结构体
	type OutboundDetourConfig struct {
		Protocol      string           `json:"protocol"`  --> 'freedom'
		Settings      *json.RawMessage `json:"settings"` --> {}
	}
*/
func (c *OutboundDetourConfig) Build() (*core.OutboundHandlerConfig, error) {

	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto proto结构体
	senderSettings := &proxyman.SenderConfig{}

	if c.SendThrough != nil {
		address := c.SendThrough
		if address.Family().IsDomain() {
			return nil, newError("unable to send through: " + address.String())
		}
		senderSettings.Via = address.Build()
	}

	if c.StreamSetting != nil {
		ss, err := c.StreamSetting.Build()
		if err != nil {
			return nil, err
		}
		if ss.SecurityType == serial.GetMessageType(&xtls.Config{}) && !strings.EqualFold(c.Protocol, "vless") {
			return nil, newError("XTLS only supports VLESS for now.")
		}
		senderSettings.StreamSettings = ss
	}

	if c.ProxySettings != nil {
		ps, err := c.ProxySettings.Build()
		if err != nil {
			return nil, newError("invalid outbound detour proxy settings.").Base(err)
		}
		senderSettings.ProxySettings = ps
	}

	if c.MuxSettings != nil {
		ms := c.MuxSettings.Build()
		if ms != nil && ms.Enabled {
			if ss := senderSettings.StreamSettings; ss != nil {
				if ss.SecurityType == serial.GetMessageType(&xtls.Config{}) {
					return nil, newError("XTLS doesn't support Mux for now.")
				}
			}
		}
		senderSettings.MultiplexSettings = ms
	}

	settings := []byte("{}") // 相当于这个值
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}

	/* rawConfig
	type FreedomConfig struct {
		DomainStrategy string  `json:"domainStrategy"`
		Timeout        *uint32 `json:"timeout"`
		Redirect       string  `json:"redirect"`
		UserLevel      uint32  `json:"userLevel"`
	}
	*/
	rawConfig, err := outboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, newError("failed to parse to outbound detour config.").Base(err)
	}
	
	/**
		ts: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/config.proto proto结构体
		ts: {
			DomainStrategy: freedom.Config_AS_IS 枚举值,
			UserLevel: 0,
		}
	*/
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, err
	}

	/**
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
	*/
	return &core.OutboundHandlerConfig{
		SenderSettings: serial.ToTypedMessage(senderSettings),
		Tag:            c.Tag,
		ProxySettings:  serial.ToTypedMessage(ts),
	}, nil
}

type StatsConfig struct{}

// Build implements Buildable.
func (c *StatsConfig) Build() (*stats.Config, error) {
	return &stats.Config{}, nil
}

type Config struct {
	Port            uint16                 `json:"port"` // Port of this Point server. Deprecated.
	LogConfig       *LogConfig             `json:"log"`
	RouterConfig    *RouterConfig          `json:"routing"`
	DNSConfig       *DnsConfig             `json:"dns"`
	InboundConfigs  []InboundDetourConfig  `json:"inbounds"`
	OutboundConfigs []OutboundDetourConfig `json:"outbounds"`

	InboundConfig   *InboundDetourConfig   `json:"inbound"`        // Deprecated.
	OutboundConfig  *OutboundDetourConfig  `json:"outbound"`       // Deprecated.
	InboundDetours  []InboundDetourConfig  `json:"inboundDetour"`  // Deprecated.
	OutboundDetours []OutboundDetourConfig `json:"outboundDetour"` // Deprecated.

	Transport       *TransportConfig       `json:"transport"`
	Policy          *PolicyConfig          `json:"policy"`
	Api             *ApiConfig             `json:"api"`
	Stats           *StatsConfig           `json:"stats"`
	Reverse         *ReverseConfig         `json:"reverse"`
}

func (c *Config) findInboundTag(tag string) int {
	found := -1
	for idx, ib := range c.InboundConfigs {
		if ib.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

func (c *Config) findOutboundTag(tag string) int {
	found := -1
	for idx, ob := range c.OutboundConfigs {
		if ob.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

// Override method accepts another Config overrides the current attribute
func (c *Config) Override(o *Config, fn string) {

	// only process the non-deprecated members

	if o.LogConfig != nil {
		c.LogConfig = o.LogConfig
	}
	if o.RouterConfig != nil {
		c.RouterConfig = o.RouterConfig
	}
	if o.DNSConfig != nil {
		c.DNSConfig = o.DNSConfig
	}
	if o.Transport != nil {
		c.Transport = o.Transport
	}
	if o.Policy != nil {
		c.Policy = o.Policy
	}
	if o.Api != nil {
		c.Api = o.Api
	}
	if o.Stats != nil {
		c.Stats = o.Stats
	}
	if o.Reverse != nil {
		c.Reverse = o.Reverse
	}

	// deprecated attrs... keep them for now
	if o.InboundConfig != nil {
		c.InboundConfig = o.InboundConfig
	}
	if o.OutboundConfig != nil {
		c.OutboundConfig = o.OutboundConfig
	}
	if o.InboundDetours != nil {
		c.InboundDetours = o.InboundDetours
	}
	if o.OutboundDetours != nil {
		c.OutboundDetours = o.OutboundDetours
	}
	// deprecated attrs

	// update the Inbound in slice if the only one in overide config has same tag
	if len(o.InboundConfigs) > 0 {
		if len(c.InboundConfigs) > 0 && len(o.InboundConfigs) == 1 {
			if idx := c.findInboundTag(o.InboundConfigs[0].Tag); idx > -1 {
				c.InboundConfigs[idx] = o.InboundConfigs[0]
				ctllog.Println("[", fn, "] updated inbound with tag: ", o.InboundConfigs[0].Tag)
			} else {
				c.InboundConfigs = append(c.InboundConfigs, o.InboundConfigs[0])
				ctllog.Println("[", fn, "] appended inbound with tag: ", o.InboundConfigs[0].Tag)
			}
		} else {
			c.InboundConfigs = o.InboundConfigs
		}
	}

	// update the Outbound in slice if the only one in overide config has same tag
	if len(o.OutboundConfigs) > 0 {
		if len(c.OutboundConfigs) > 0 && len(o.OutboundConfigs) == 1 {
			if idx := c.findOutboundTag(o.OutboundConfigs[0].Tag); idx > -1 {
				c.OutboundConfigs[idx] = o.OutboundConfigs[0]
				ctllog.Println("[", fn, "] updated outbound with tag: ", o.OutboundConfigs[0].Tag)
			} else {
				if strings.Contains(strings.ToLower(fn), "tail") {
					c.OutboundConfigs = append(c.OutboundConfigs, o.OutboundConfigs[0])
					ctllog.Println("[", fn, "] appended outbound with tag: ", o.OutboundConfigs[0].Tag)
				} else {
					c.OutboundConfigs = append(o.OutboundConfigs, c.OutboundConfigs...)
					ctllog.Println("[", fn, "] prepended outbound with tag: ", o.OutboundConfigs[0].Tag)
				}
			}
		} else {
			c.OutboundConfigs = o.OutboundConfigs
		}
	}
}

func applyTransportConfig(s *StreamConfig, t *TransportConfig) {
	if s.TCPSettings == nil {
		s.TCPSettings = t.TCPConfig
	}
	if s.KCPSettings == nil {
		s.KCPSettings = t.KCPConfig
	}
	if s.WSSettings == nil {
		s.WSSettings = t.WSConfig
	}
	if s.HTTPSettings == nil {
		s.HTTPSettings = t.HTTPConfig
	}
	if s.DSSettings == nil {
		s.DSSettings = t.DSConfig
	}
}

// Build implements Buildable.
// 现在还是在v2ctrl里面
/**
{
  "inbounds": [
    {
      "port": 10086,
      "protocol": "vmess",
      "settings": {
        "clients": [
          {
            "id": "b831381d-6324-4d53-ad4f-8cda48b30811"
          }
        ]
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "settings": {}
    }
  ]
}
	c是json结构体, 调用时只有 InboundConfigs OutboundConfigs 有值
	 	Config json结构体 {
			...
		InboundConfigs  []InboundDetourConfig  `json:"inbounds"`
		OutboundConfigs []OutboundDetourConfig `json:"outbounds"`
			...
	}
**/

func (c *Config) Build() (*core.Config, error) {
	config := &core.Config{
		App: []*serial.TypedMessage{
			// proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/dispatcher/config.proto
			serial.ToTypedMessage(&dispatcher.Config{}),

			// proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
			serial.ToTypedMessage(&proxyman.InboundConfig{}),

			// proto生成的结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/config.proto
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
		},
	}

	// 没配置c.Api
	if c.Api != nil {
		apiConf, err := c.Api.Build()
		if err != nil {
			return nil, err
		}
		config.App = append(config.App, serial.ToTypedMessage(apiConf))
	}

	if c.Stats != nil {
		statsConf, err := c.Stats.Build()
		if err != nil {
			return nil, err
		}
		config.App = append(config.App, serial.ToTypedMessage(statsConf))
	}

	var logConfMsg *serial.TypedMessage
	if c.LogConfig != nil {
		logConfMsg = serial.ToTypedMessage(c.LogConfig.Build())
	} else {
		logConfMsg = serial.ToTypedMessage(DefaultLogConfig())
	}
	// let logger module be the first App to start,
	// so that other modules could print log during initiating
	config.App = append([]*serial.TypedMessage{logConfMsg}, config.App...)

	/** 此时的config:
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
		]
	}
	**/

	if c.RouterConfig != nil {
		routerConfig, err := c.RouterConfig.Build()
		if err != nil {
			return nil, err
		}
		config.App = append(config.App, serial.ToTypedMessage(routerConfig))
	}

	if c.DNSConfig != nil {
		dnsApp, err := c.DNSConfig.Build()
		if err != nil {
			return nil, newError("failed to parse DNS config").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(dnsApp))
	}

	if c.Policy != nil {
		pc, err := c.Policy.Build()
		if err != nil {
			return nil, err
		}
		config.App = append(config.App, serial.ToTypedMessage(pc))
	}

	if c.Reverse != nil {
		r, err := c.Reverse.Build()
		if err != nil {
			return nil, err
		}
		config.App = append(config.App, serial.ToTypedMessage(r))
	}

	var inbounds []InboundDetourConfig

	if c.InboundConfig != nil {
		inbounds = append(inbounds, *c.InboundConfig)
	}

	if len(c.InboundDetours) > 0 {
		inbounds = append(inbounds, c.InboundDetours...)
	}

	// 只配置了这个
	if len(c.InboundConfigs) > 0 {
		inbounds = append(inbounds, c.InboundConfigs...)
	}
	/**
		此时的inbounds, 此时这三个有值
	type InboundDetourConfig struct {
		Protocol       string                         `json:"protocol"`  --> 'vmess'
		PortRange      *PortRange                     `json:"port"` // 这个是自定义实现了unmarshal方法 --> 10086
		Settings       *json.RawMessage               `json:"settings"` : { clients: [ { id: xxx } ] }
	}
	**/

	// Backward compatibility.
	if len(inbounds) > 0 && inbounds[0].PortRange == nil && c.Port > 0 {
		inbounds[0].PortRange = &PortRange{
			From: uint32(c.Port),
			To:   uint32(c.Port),
		}
	}

	// 只有一个inbounds
	for _, rawInboundConfig := range inbounds {
		if c.Transport != nil {
			if rawInboundConfig.StreamSetting == nil {
				rawInboundConfig.StreamSetting = &StreamConfig{}
			}
			applyTransportConfig(rawInboundConfig.StreamSetting, c.Transport)
		}

		/** 生成proto 结构体
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

		*/
		ic, err := rawInboundConfig.Build()
		if err != nil {
			return nil, err
		}
		config.Inbound = append(config.Inbound, ic)
	}

	/** 此时的config:
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

		]
	}
	**/


	var outbounds []OutboundDetourConfig

	// nil
	if c.OutboundConfig != nil {
		outbounds = append(outbounds, *c.OutboundConfig)
	}

	// = 0
	if len(c.OutboundDetours) > 0 {
		outbounds = append(outbounds, c.OutboundDetours...)
	}

	if len(c.OutboundConfigs) > 0 {
		outbounds = append(outbounds, c.OutboundConfigs...)
	}
	// 此时的outbounds, 只有两个有值
	/** [
	type OutboundDetourConfig struct {
		Protocol      string           `json:"protocol"`  --> 'freedom'
		Settings      *json.RawMessage `json:"settings"` --> {}
	}
	]
	*/

	for _, rawOutboundConfig := range outbounds {
		if c.Transport != nil {
			if rawOutboundConfig.StreamSetting == nil {
				rawOutboundConfig.StreamSetting = &StreamConfig{}
			}
			applyTransportConfig(rawOutboundConfig.StreamSetting, c.Transport)
		}

			/**
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
	*/
		oc, err := rawOutboundConfig.Build()
		if err != nil {
			return nil, err
		}
		config.Outbound = append(config.Outbound, oc)
	}

	/** 此时的config:
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
	return config, nil
}
