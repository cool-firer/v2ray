package conf

import (
	"encoding/json"
	"strings"
	"fmt"
	"os"

	"github.com/golang/protobuf/proto"

	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/proxy/vmess"
	"v2ray.com/core/proxy/vmess/inbound"
	"v2ray.com/core/proxy/vmess/outbound"
)

type VMessAccount struct {
	ID       string `json:"id"`
	AlterIds uint16 `json:"alterId"`
	Security string `json:"security"`
}

// Build implements Buildable
/**
	a: {
		ID: 'b831381d-6324-4d53-ad4f-8cda48b30811'
		其他是默认值
	}
**/
func (a *VMessAccount) Build() *vmess.Account {
	var st protocol.SecurityType // proto生成的

	switch strings.ToLower(a.Security) {
	case "aes-128-gcm":
		st = protocol.SecurityType_AES128_GCM
	case "chacha20-poly1305":
		st = protocol.SecurityType_CHACHA20_POLY1305
	case "auto":
		st = protocol.SecurityType_AUTO
	case "none":
		st = protocol.SecurityType_NONE
	default:
		// 默认
		st = protocol.SecurityType_AUTO
	}
	return &vmess.Account{ // proto生成的结构体
		Id:      a.ID,
		AlterId: uint32(a.AlterIds), // 默认0
		SecuritySettings: &protocol.SecurityConfig{
			Type: st,  // SecurityType_AUTO
		},
	}
}

type VMessDetourConfig struct {
	ToTag string `json:"to"`
}

// Build implements Buildable
func (c *VMessDetourConfig) Build() *inbound.DetourConfig {
	return &inbound.DetourConfig{
		To: c.ToTag,
	}
}

type FeaturesConfig struct {
	Detour *VMessDetourConfig `json:"detour"`
}

type VMessDefaultConfig struct {
	AlterIDs uint16 `json:"alterId"`
	Level    byte   `json:"level"`
}

// Build implements Buildable
func (c *VMessDefaultConfig) Build() *inbound.DefaultConfig {
	config := new(inbound.DefaultConfig)
	config.AlterId = uint32(c.AlterIDs)
	if config.AlterId == 0 {
		config.AlterId = 32
	}
	config.Level = uint32(c.Level)
	return config
}

type VMessInboundConfig struct {
	Users        []json.RawMessage   `json:"clients"`
	Features     *FeaturesConfig     `json:"features"`
	Defaults     *VMessDefaultConfig `json:"default"`
	DetourConfig *VMessDetourConfig  `json:"detour"`
	SecureOnly   bool                `json:"disableInsecureEncryption"`
}

// Build implements Buildable
/**
	c: 只有Users有值 {
			Users        []json.RawMessage   `json:"clients"` [ {id: xxx}, ] [  二进制流,  ] 用数组包裹的二进制流
			Features     *FeaturesConfig     `json:"features"`
			Defaults     *VMessDefaultConfig `json:"default"`
			DetourConfig *VMessDetourConfig  `json:"detour"`
			SecureOnly   bool                `json:"disableInsecureEncryption"`
	}
**/
func (c *VMessInboundConfig) Build() (proto.Message, error) {

	// config /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/config.proto 生成的结构体
	config := &inbound.Config{
		SecureEncryptionOnly: c.SecureOnly, // false
	}

	// false
	if c.Defaults != nil {
		config.Default = c.Defaults.Build()
	}

	// false
	if c.DetourConfig != nil {
		config.Detour = c.DetourConfig.Build()
	} else if c.Features != nil && c.Features.Detour != nil {
		config.Detour = c.Features.Detour.Build()
	}

	// protocol.User: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/protocol/user.proto 生成的结构体
	// config.User引用的 user proto
	config.User = make([]*protocol.User, len(c.Users))

	// fmt.Printf("---vmess c:%+v\n", c)
	fmt.Fprintln(os.Stderr, "---vmess c:%+v\n", c.Users)

	for idx, rawData := range c.Users {

		// 空的user, 没有id字段 proto生成的结构体
		user := new(protocol.User)
		if err := json.Unmarshal(rawData, user); err != nil {
			return nil, newError("invalid VMess user").Base(err)
		}
		// fmt.Fprintln(os.Stderr, "---vmess user :%+v\n", *user)
		// 0, ''
		// fmt.Fprintln(os.Stderr, "level, email", user.Level, user.Email)

		// id原来在account里面, account里面也只有id VMessAccount是普通结构体
		account := new(VMessAccount)
		if err := json.Unmarshal(rawData, account); err != nil {
			return nil, newError("invalid VMess user").Base(err)
		}
		// b831381d-6324-4d53-ad4f-8cda48b30811
		fmt.Fprintln(os.Stderr, "id: ", account.ID)

		// account.Bild() 生成 proto结构体: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/account.proto
		user.Account = serial.ToTypedMessage(account.Build())
		// user.Account = typedMessage: { type:'v2ray.core.proxy.vemss.Account' value: []byte二进制 }

		config.User[idx] = user
	}
	/**
	config: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/vmess/inbound/config.proto 生成的结构体
	User: /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/protocol/user.proto 生成的结构体

 		最终的config(proto生成的结构体): {
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
	return config, nil
}

type VMessOutboundTarget struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}
type VMessOutboundConfig struct {
	Receivers []*VMessOutboundTarget `json:"vnext"`
}

// Build implements Buildable
func (c *VMessOutboundConfig) Build() (proto.Message, error) {
	config := new(outbound.Config)

	if len(c.Receivers) == 0 {
		return nil, newError("0 VMess receiver configured")
	}
	serverSpecs := make([]*protocol.ServerEndpoint, len(c.Receivers))
	for idx, rec := range c.Receivers {
		if len(rec.Users) == 0 {
			return nil, newError("0 user configured for VMess outbound")
		}
		if rec.Address == nil {
			return nil, newError("address is not set in VMess outbound config")
		}
		spec := &protocol.ServerEndpoint{
			Address: rec.Address.Build(),
			Port:    uint32(rec.Port),
		}
		for _, rawUser := range rec.Users {
			user := new(protocol.User)
			if err := json.Unmarshal(rawUser, user); err != nil {
				return nil, newError("invalid VMess user").Base(err)
			}
			account := new(VMessAccount)
			if err := json.Unmarshal(rawUser, account); err != nil {
				return nil, newError("invalid VMess user").Base(err)
			}
			user.Account = serial.ToTypedMessage(account.Build())
			spec.User = append(spec.User, user)
		}
		serverSpecs[idx] = spec
	}
	config.Receiver = serverSpecs
	return config, nil
}
