// +build !confonly

package freedom

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"context"
	"time"
	"fmt"

	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/retry"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/signal"
	"v2ray.com/core/common/task"
	"v2ray.com/core/features/dns"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		
		// in freedom.go Config, ctx:context.Background.WithValue(type core.V2rayKey, val <not Stringer>)  config:
		fmt.Printf("in /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go Config, ctx:%+v  config:%+v\n", ctx, config)
		h := new(Handler)

		/**
		type Handler struct {
			policyManager policy.Manager
			dns           dns.Client
			config        *Config
		}
		*/

		// 会调用这个函数
		// r封装了/Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go 的callback,
		if err := core.RequireFeatures(ctx, func(pm policy.Manager, d dns.Client) error {
			return h.Init(config.(*Config), pm, d)
		}); err != nil {
			return nil, err
		}
		return h, nil
	}))
}

// Handler handles Freedom connections.
type Handler struct {
	policyManager policy.Manager
	dns           dns.Client
	config        *Config
}

// Init initializes the Handler with necessary parameters.
func (h *Handler) Init(config *Config, pm policy.Manager, d dns.Client) error {
	h.config = config
	h.policyManager = pm
	h.dns = d

	return nil
}

// 	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/features/policy/policy.go
func (h *Handler) policy() policy.Session {
	p := h.policyManager.ForLevel(h.config.UserLevel)
	if h.config.Timeout > 0 && h.config.UserLevel == 0 {
		p.Timeouts.ConnectionIdle = time.Duration(h.config.Timeout) * time.Second
	}
	return p
}

func (h *Handler) resolveIP(ctx context.Context, domain string, localAddr net.Address) net.Address {
	var lookupFunc func(string) ([]net.IP, error) = h.dns.LookupIP

	if h.config.DomainStrategy == Config_USE_IP4 || (localAddr != nil && localAddr.Family().IsIPv4()) {
		if lookupIPv4, ok := h.dns.(dns.IPv4Lookup); ok {
			lookupFunc = lookupIPv4.LookupIPv4
		}
	} else if h.config.DomainStrategy == Config_USE_IP6 || (localAddr != nil && localAddr.Family().IsIPv6()) {
		if lookupIPv6, ok := h.dns.(dns.IPv6Lookup); ok {
			lookupFunc = lookupIPv6.LookupIPv6
		}
	}

	ips, err := lookupFunc(domain)
	if err != nil {
		newError("failed to get IP address for domain ", domain).Base(err).WriteToLog(session.ExportIDToError(ctx))
	}
	if len(ips) == 0 {
		return nil
	}
	return net.IPAddress(ips[dice.Roll(len(ips))])
}

func isValidAddress(addr *net.IPOrDomain) bool {
	if addr == nil {
		return false
	}

	a := addr.AsAddress()
	return a != net.AnyIP
}

// Process implements proxy.Outbound.
// go程 运行
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	// { Target: 有值 }
	outbound := session.OutboundFromContext(ctx)
	if outbound == nil || !outbound.Target.IsValid() {
		return newError("target not specified.")
	}

	// 10.211.55.2:8080
	destination := outbound.Target

	if h.config.DestinationOverride != nil {
		server := h.config.DestinationOverride.Server
		if isValidAddress(server.Address) {
			destination.Address = server.Address.AsAddress()
		}
		if server.Port != 0 {
			destination.Port = net.Port(server.Port)
		}
	}
	newError("opening connection to ", destination).WriteToLog(session.ExportIDToError(ctx))

	fmt.Println("[Sub]          freedom opening connection to ", destination, " /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/freedom/freedom.go")

	input := link.Reader
	output := link.Writer

	var conn internet.Connection

	// 指数退让 返回retry实例
	// On: 调用func, 最多50次 time.Sleep
	err := retry.ExponentialBackoff(5, 100).On(func() error {
		dialDest := destination
		if h.config.useIP() && dialDest.Address.Family().IsDomain() {
			ip := h.resolveIP(ctx, dialDest.Address.Domain(), dialer.Address())
			if ip != nil {
				dialDest = net.Destination{
					Network: dialDest.Network,
					Address: ip,
					Port:    dialDest.Port,
				}
				newError("dialing to to ", dialDest).WriteToLog(session.ExportIDToError(ctx))
			}
		}

		// 连接到目标
		rawConn, err := dialer.Dial(ctx, dialDest)
		if err != nil {
			return err
		}
		conn = rawConn
		return nil
	})

	if err != nil {
		return newError("failed to open connection to ", destination).Base(err)
	}

	defer conn.Close() // nolint: errcheck

	plcy := h.policy()
	ctx, cancel := context.WithCancel(ctx)
	
	// ConnectionIdle: time.Second * 300
	timer := signal.CancelAfterInactivity(ctx, cancel, plcy.Timeouts.ConnectionIdle)

	requestDone := func() error {
		defer timer.SetTimeout(plcy.Timeouts.DownlinkOnly)

		var writer buf.Writer
		if destination.Network == net.Network_TCP {

		/**
			writer: &BufferToBytesWriter{
				Writer: conn的io.Writer抽象接口
			}
		*/
			writer = buf.NewWriter(conn)
		} else {
			writer = &buf.SequentialWriter{Writer: conn}
		}

		// 从Reader读, 写入conn		
		if err := buf.Copy(input, writer, buf.UpdateActivity(timer), buf.SetGoType("[freedom - request]")); err != nil {
			fmt.Println("[freedom - request] returned with err:", err)
			return newError("failed to process request").Base(err)
		}
		fmt.Println("[freedom - request] returned")

		return nil
	}

	responseDone := func() error {
		defer timer.SetTimeout(plcy.Timeouts.UplinkOnly)

		var reader buf.Reader
		if destination.Network == net.Network_TCP {
			reader = buf.NewReader(conn)
		} else {
			reader = buf.NewPacketReader(conn)
		}
		if err := buf.Copy(reader, output, buf.UpdateActivity(timer), buf.SetGoType("[freedom - response]")); err != nil {
			fmt.Println("[freedom - response] returned with err:", err)
			return newError("failed to process response").Base(err)
		}
		fmt.Println("[freedom - response] returned")

		return nil
	}

	// 调试用
	// responseDone = func() error { 
	// 	fmt.Println("[freedom] responseDone")
	// 	return nil
	// }
	// requestDone = func() error { 
	// 	fmt.Println("[freedom] requestDone")
	// 	return nil
	// }

	// fmt.Println("[freendom] output:", output)

	// 起两个go跑requestDoen、OnSuccess
	// 阻塞等待两个跑完的结果
	if err := task.Run(ctx, requestDone, task.OnSuccess(responseDone, task.Close(output))); err != nil {
		fmt.Println("[Sub] Run returned with err:", err)
		return newError("connection ends").Base(err)
	}
	fmt.Println("[Sub] Run returned")

	return nil
}
