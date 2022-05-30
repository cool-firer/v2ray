package conf

import (
	"encoding/json"
	"strings"
)

type ConfigCreator func() interface{}

type ConfigCreatorCache map[string]ConfigCreator

func (v ConfigCreatorCache) RegisterCreator(id string, creator ConfigCreator) error {
	if _, found := v[id]; found {
		return newError(id, " already registered.").AtError()
	}

	v[id] = creator
	return nil
}

func (v ConfigCreatorCache) CreateConfig(id string) (interface{}, error) {
	creator, found := v[id]
	if !found {
		return nil, newError("unknown config id: ", id)
	}
	// creator()调用返回: new(VMessInboundConfig)
	return creator(), nil
}

type JSONConfigLoader struct {
	cache     ConfigCreatorCache
	idKey     string
	configKey string
}

// cache: map[string][func] 
// idKey: "protocol"
// configKey: "settings"
func NewJSONConfigLoader(cache ConfigCreatorCache, idKey string, configKey string) *JSONConfigLoader {
	return &JSONConfigLoader{
		idKey:     idKey, // 'protocol'
		configKey: configKey, // 'settings'
		cache:     cache,
	}
}

/**
settings raw: byte切片 {  
	clients: [
	 { "id": "b831381d-6324-4d53-ad4f-8cda48b30811" }		 
	] 
}
id: : 'vmess'
*/
func (v *JSONConfigLoader) LoadWithID(raw []byte, id string) (interface{}, error) {
	id = strings.ToLower(id) // 'vmess'

	/**
	config = VMessInboundConfig 普通结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/conf/vmess.go
	type VMessInboundConfig struct {
			Users        []json.RawMessage   `json:"clients"`
			Features     *FeaturesConfig     `json:"features"`
			Defaults     *VMessDefaultConfig `json:"default"`
			DetourConfig *VMessDetourConfig  `json:"detour"`
			SecureOnly   bool                `json:"disableInsecureEncryption"`
	}


	id是http时 new一个普通结构体 /Users/demon/Desktop/work/gowork/src/v2ray.com/core/infra/conf/http.go
	type HttpServerConfig struct {
		Timeout     uint32         `json:"timeout"`
		Accounts    []*HttpAccount `json:"accounts"`
		Transparent bool           `json:"allowTransparent"`
		UserLevel   uint32         `json:"userLevel"`
	}

	**/
	config, err := v.cache.CreateConfig(id) // CreateConfig返回: new(VMessInboundConfig)
	if err != nil {
		return nil, err
	}

	// 填充值
	if err := json.Unmarshal(raw, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (v *JSONConfigLoader) Load(raw []byte) (interface{}, string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, "", err
	}
	rawID, found := obj[v.idKey]
	if !found {
		return nil, "", newError(v.idKey, " not found in JSON context").AtError()
	}
	var id string
	if err := json.Unmarshal(rawID, &id); err != nil {
		return nil, "", err
	}
	rawConfig := json.RawMessage(raw)
	if len(v.configKey) > 0 {
		configValue, found := obj[v.configKey]
		if found {
			rawConfig = configValue
		} else {
			// Default to empty json object.
			rawConfig = json.RawMessage([]byte("{}"))
		}
	}
	config, err := v.LoadWithID([]byte(rawConfig), id)
	if err != nil {
		return nil, id, err
	}
	return config, id, nil
}
