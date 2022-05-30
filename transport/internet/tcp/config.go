// +build !confonly

package tcp

import (
	"fmt"
	"v2ray.com/core/common"
	"v2ray.com/core/transport/internet"
)

const protocolName = "tcp"

func init() {
	common.Must(internet.RegisterProtocolConfigCreator(protocolName, func() interface{} {
		fmt.Println("  in /Users/demon/Desktop/work/gowork/src/v2ray.com/core/transport/internet/tcp/config.go protocolName:", protocolName)
		return new(Config)
	}))
}
