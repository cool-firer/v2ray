// +build !confonly

package dispatcher

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"context"
	"strings"
	"sync"
	"time"
	"fmt"

	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/log"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/session"
	"v2ray.com/core/features/outbound"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/features/routing"
	routing_session "v2ray.com/core/features/routing/session"
	"v2ray.com/core/features/stats"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/pipe"
)

var (
	errSniffingTimeout = newError("timeout on sniffing")
)

type cachedReader struct {
	sync.Mutex
	reader *pipe.Reader
	cache  buf.MultiBuffer
}

func (r *cachedReader) Cache(b *buf.Buffer) {
	mb, _ := r.reader.ReadMultiBufferTimeout(time.Millisecond * 100)
	r.Lock()
	if !mb.IsEmpty() {
		r.cache, _ = buf.MergeMulti(r.cache, mb)
	}
	b.Clear()
	rawBytes := b.Extend(buf.Size)
	n := r.cache.Copy(rawBytes) // copy进rawBytes
	b.Resize(0, int32(n))
	r.Unlock()
}

func (r *cachedReader) readInternal() buf.MultiBuffer {
	r.Lock()
	defer r.Unlock()

	if r.cache != nil && !r.cache.IsEmpty() {
		mb := r.cache
		r.cache = nil
		return mb
	}

	return nil
}

func (r *cachedReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb := r.readInternal()
	if mb != nil {
		return mb, nil
	}

	return r.reader.ReadMultiBuffer()
}

func (r *cachedReader) ReadMultiBufferTimeout(timeout time.Duration) (buf.MultiBuffer, error) {
	mb := r.readInternal()
	if mb != nil {
		return mb, nil
	}

	return r.reader.ReadMultiBufferTimeout(timeout)
}

func (r *cachedReader) Interrupt() {
	r.Lock()
	if r.cache != nil {
		r.cache = buf.ReleaseMulti(r.cache)
	}
	r.Unlock()
	r.reader.Interrupt()
}

// DefaultDispatcher is a default implementation of Dispatcher.
type DefaultDispatcher struct {
	ohm    outbound.Manager
	router routing.Router
	policy policy.Manager
	stats  stats.Manager
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		
		// in default.go, ctx:context.Background.WithValue(type core.V2rayKey, val <not Stringer>)  config:
		fmt.Printf("  in default.go, ctx:%+v  config:%+v\n", ctx, config)
		d := new(DefaultDispatcher)

		
		if err := core.RequireFeatures(ctx, func(om outbound.Manager, router routing.Router, pm policy.Manager, sm stats.Manager) error {
			
			fmt.Println("Fuck Ass Code")
			
			return d.Init(config.(*Config), om, router, pm, sm)
		}); err != nil {
			return nil, err
		}


		return d, nil
	}))
}

// Init initializes DefaultDispatcher.
func (d *DefaultDispatcher) Init(config *Config, om outbound.Manager, router routing.Router, pm policy.Manager, sm stats.Manager) error {
	d.ohm = om
	d.router = router
	d.policy = pm
	d.stats = sm
	return nil
}

// Type implements common.HasType.
func (*DefaultDispatcher) Type() interface{} {
	return routing.DispatcherType()
}

// Start implements common.Runnable.
func (*DefaultDispatcher) Start() error {
	return nil
}

// Close implements common.Closable.
func (*DefaultDispatcher) Close() error { return nil }

func (d *DefaultDispatcher) getLink(ctx context.Context) (*transport.Link, *transport.Link) {
	
	// []Option: 一个函数
	opt := pipe.OptionsFromContext(ctx)

	/**
		&Reader{
			pipe: &pipe{
				readSignal: &Notifier{
					c: make(chan struct{}, 1),
				},
				writeSignal: &Notifier{
					c: make(chan struct{}, 1),
				},
				done: 		&Instance{
					c: make(chan struct{}),
				},
				option: pipeOption{
					limit: -1,
				},
			},
		}

		&Writer{
			pipe: 跟reader同一个p
		}
	*/

	uplinkReader, uplinkWriter := pipe.New(opt...)

	downlinkReader, downlinkWriter := pipe.New(opt...)

	inboundLink := &transport.Link{
		Reader: downlinkReader,
		Writer: uplinkWriter,
	}

	outboundLink := &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}

	sessionInbound := session.InboundFromContext(ctx)
	var user *protocol.MemoryUser
	if sessionInbound != nil {
		// &MemoryUser: { Level: 0 }
		user = sessionInbound.User
	}

	if user != nil && len(user.Email) > 0 {
		p := d.policy.ForLevel(user.Level)
		if p.Stats.UserUplink {
			name := "user>>>" + user.Email + ">>>traffic>>>uplink"
			if c, _ := stats.GetOrRegisterCounter(d.stats, name); c != nil {
				inboundLink.Writer = &SizeStatWriter{
					Counter: c,
					Writer:  inboundLink.Writer,
				}
			}
		}
		if p.Stats.UserDownlink {
			name := "user>>>" + user.Email + ">>>traffic>>>downlink"
			if c, _ := stats.GetOrRegisterCounter(d.stats, name); c != nil {
				outboundLink.Writer = &SizeStatWriter{
					Counter: c,
					Writer:  outboundLink.Writer,
				}
			}
		}
	}

	return inboundLink, outboundLink
}

func shouldOverride(result SniffResult, domainOverride []string) bool {
	for _, p := range domainOverride {
		if strings.HasPrefix(result.Protocol(), p) {
			return true
		}
	}
	return false
}

// Dispatch implements routing.Dispatcher.
func (d *DefaultDispatcher) Dispatch(ctx context.Context, destination net.Destination) (*transport.Link, error) {
	
	fmt.Println("    in Dispatch /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/dispatcher/default.go")
	
	if !destination.IsValid() {
		panic("Dispatcher: Invalid destination.")
	}
	ob := &session.Outbound{
		Target: destination,
	}
	ctx = session.ContextWithOutbound(ctx, ob)

	inbound, outbound := d.getLink(ctx)


	content := session.ContentFromContext(ctx)
	if content == nil {
		content = new(session.Content)
		ctx = session.ContextWithContent(ctx, content)
	}

	sniffingRequest := content.SniffingRequest
	fmt.Println("    destination:", destination, " destination.Network:", destination.Network, " sniffingRequest:", sniffingRequest, " Enabled:", sniffingRequest.Enabled)
	if destination.Network != net.Network_TCP || !sniffingRequest.Enabled {
		/**
			outbound: &transport.Link{
				Reader: &Reader{
									pipe: &pipe{
									readSignal: &Notifier{
										c: make(chan struct{}, 1),
									},
									writeSignal: &Notifier{
										c: make(chan struct{}, 1),
									},
									done: 		&Instance{
										c: make(chan struct{}),
									},
									option: pipeOption{
										limit: -1,
									},
								},
				},

				Writer: downlinkWriter,
			}
		*/
		// 连接到到目标的tcp, 开两个go后，阻塞等待两个go跑完 
		go d.routedDispatch(ctx, outbound, destination)
	} else {
		go func() {
			cReader := &cachedReader{
				reader: outbound.Reader.(*pipe.Reader),
			}
			outbound.Reader = cReader
			result, err := sniffer(ctx, cReader)
			if err == nil {
				content.Protocol = result.Protocol()
			}
			if err == nil && shouldOverride(result, sniffingRequest.OverrideDestinationForProtocol) {
				domain := result.Domain()
				newError("sniffed domain: ", domain).WriteToLog(session.ExportIDToError(ctx))
				destination.Address = net.ParseAddress(domain)
				ob.Target = destination
			}
			d.routedDispatch(ctx, outbound, destination)
		}()
	}
	return inbound, nil
}

func sniffer(ctx context.Context, cReader *cachedReader) (SniffResult, error) {
	payload := buf.New()
	defer payload.Release()

	sniffer := NewSniffer()
	totalAttempt := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			totalAttempt++
			if totalAttempt > 2 {
				return nil, errSniffingTimeout
			}

			// 1、重置payload start,end
			// 2、把cReader.cache复制到payload里
			// 3、根据实际复制的长度, 修正payload end索引
			cReader.Cache(payload)
			if !payload.IsEmpty() {
				result, err := sniffer.Sniff(payload.Bytes())
				if err != common.ErrNoClue {
					return result, err
				}
			}
			if payload.IsFull() {
				return nil, errUnknownContent
			}
		}
	}
}

func (d *DefaultDispatcher) routedDispatch(ctx context.Context, link *transport.Link, destination net.Destination) {
	var handler outbound.Handler

	skipRoutePick := false
	if content := session.ContentFromContext(ctx); content != nil {
		skipRoutePick = content.SkipRoutePick // false
	}
	fmt.Println("[Sub]      routedDispatch d.router:", d.router, " skipRoutePick:", skipRoutePick)
	if d.router != nil && !skipRoutePick {
		if route, err := d.router.PickRoute(routing_session.AsRoutingContext(ctx)); err == nil {
			tag := route.GetOutboundTag()
			if h := d.ohm.GetHandler(tag); h != nil {
				newError("taking detour [", tag, "] for [", destination, "]").WriteToLog(session.ExportIDToError(ctx))
				handler = h
			} else {
				newError("non existing tag: ", tag).AtWarning().WriteToLog(session.ExportIDToError(ctx))
			}
		} else {
			fmt.Println("[Sub]      default route for ", destination, " error")
			newError("default route for ", destination).WriteToLog(session.ExportIDToError(ctx))
		}
	}

	if handler == nil {
		// 返回outbound.Manager的defaultHandler
		handler = d.ohm.GetDefaultHandler()
		// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/outbound.go
	}

	if handler == nil {
		newError("default outbound handler not exist").WriteToLog(session.ExportIDToError(ctx))
		common.Close(link.Writer)
		common.Interrupt(link.Reader)
		return
	}

	if accessMessage := log.AccessMessageFromContext(ctx); accessMessage != nil {
		if tag := handler.Tag(); tag != "" {
			accessMessage.Detour = tag
		}
		log.Record(accessMessage)
	}

	//  /Users/demon/Desktop/work/gowork/src/v2ray.com/core/app/proxyman/outbound/handler.go
	handler.Dispatch(ctx, link)
}
