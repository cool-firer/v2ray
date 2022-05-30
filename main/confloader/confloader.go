package confloader

import (
	"io"
	"os"
)

type configFileLoader func(string) (io.Reader, error)
type extconfigLoader func([]string) (io.Reader, error)

var (
	EffectiveConfigFileLoader configFileLoader
	EffectiveExtConfigLoader  extconfigLoader
)

// LoadConfig reads from a path/url/stdin
// actual work is in external module
func LoadConfig(file string) (io.Reader, error) {
	if EffectiveConfigFileLoader == nil {
		newError("external config module not loaded, reading from stdin").AtInfo().WriteToLog()
		return os.Stdin, nil
	}
	return EffectiveConfigFileLoader(file)
}

// LoadExtConfig calls v2ctl to handle multiple config
// the actual work also in external module
func LoadExtConfig(files []string) (io.Reader, error) {
	if EffectiveExtConfigLoader == nil {
		return nil, newError("external config module not loaded").AtError()
	}

	// 在/Users/demon/Desktop/work/gowork/src/v2ray.com/core/main/confloader/external/external.go 里面赋值
	// 真TM的服了, 是为了把人搞晕才这搞一点那搞一点的嘛？
	return EffectiveExtConfigLoader(files)
}
