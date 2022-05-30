// +build !confonly

package http

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"
	"fmt"


	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buf"
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/log"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	http_proto "v2ray.com/core/common/protocol/http"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/signal"
	"v2ray.com/core/common/task"
	"v2ray.com/core/features/policy"
	"v2ray.com/core/features/routing"
	"v2ray.com/core/transport/internet"
)

// Server is an HTTP proxy server.
type Server struct {
	config        *ServerConfig
	policyManager policy.Manager
}

// NewServer creates a new HTTP inbound handler.
func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	v := core.MustFromContext(ctx)
	s := &Server{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}

	return s, nil
}

func (s *Server) policy() policy.Session {
	config := s.config
	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/features/policy/policy.go
	p := s.policyManager.ForLevel(config.UserLevel)
	if config.Timeout > 0 && config.UserLevel == 0 {
		p.Timeouts.ConnectionIdle = time.Duration(config.Timeout) * time.Second
	}
	/**
	Session{
			Timeouts: Timeout{
				//Align Handshake timeout with nginx client_header_timeout
				//So that this value will not indicate server identity
				Handshake:      time.Second * 60,
				ConnectionIdle: time.Second * 300,
				UplinkOnly:     time.Second * 1,
				DownlinkOnly:   time.Second * 1,
			},
			Stats: Stats{
				UserUplink:   false,
				UserDownlink: false,
			},
			Buffer: defaultBufferPolicy(),
		}

	*/

	return p
}

// Network implements proxy.Inbound.
func (*Server) Network() []net.Network {
	return []net.Network{net.Network_TCP}
}

func isTimeout(err error) bool {
	nerr, ok := errors.Cause(err).(net.Error)
	return ok && nerr.Timeout()
}

func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

// 可以接收实现了Reader接口的类型, 并且只能调用reader接口
type readerOnly struct {
	io.Reader
}

func (s *Server) Process(ctx context.Context, network net.Network, conn internet.Connection, dispatcher routing.Dispatcher) error {
	/** 拿到Inbound信息
	&session.Inbound{
		Source:  net.DestinationFromAddr(conn.RemoteAddr()), // 对端的ip port
		Gateway: net.TCPDestination(w.address, w.port), // 当前监听的ip port
		Tag:     w.tag, // ''
	})
	*/
	inbound := session.InboundFromContext(ctx)

	if inbound != nil {
		inbound.User = &protocol.MemoryUser{
			Level: s.config.UserLevel,
		}
	}

	// buf.Size: 2048
	// 返回一个新的Reader结构体, 带有 []2048缓冲
	reader := bufio.NewReaderSize(readerOnly{conn}, buf.Size)

Start:
	// s.policy().Timeouts.Handshake: time.Second * 60
	if err := conn.SetReadDeadline(time.Now().Add(s.policy().Timeouts.Handshake)); err != nil {
		newError("failed to set read deadline").Base(err).WriteToLog(session.ExportIDToError(ctx))
	}

	// ReadRequest reads and parses an incoming request from b. 解析成http请求
	// 这里是不是会阻塞? 有请求了才继续往下
	request, err := http.ReadRequest(reader)
	if err != nil {
		trace := newError("failed to read http request").Base(err)
		if errors.Cause(err) != io.EOF && !isTimeout(errors.Cause(err)) {
			trace.AtWarning() // nolint: errcheck
		}
		return trace
	}

	if len(s.config.Accounts) > 0 {
		user, pass, ok := parseBasicAuth(request.Header.Get("Proxy-Authorization"))
		if !ok || !s.config.HasAccount(user, pass) {
			return common.Error2(conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"proxy\"\r\nConnection: close\r\n\r\n")))
		}
		if inbound != nil {
			inbound.User.Email = user
		}
	}

	newError("request to Method [", request.Method, "] Host [", request.Host, "] with URL [", request.URL, "]").WriteToLog(session.ExportIDToError(ctx))
	// ===request to Method [ CONNECT ] Host [ 10.211.55.2:8080 ] with URL [ //10.211.55.2:8080 ]===
	fmt.Println("===request to Method [", request.Method, "] Host [", request.Host, "] with URL [", request.URL, "] header:", request.Header, "===")
	
	// A zero value for t means Read will not time out.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		newError("failed to clear read deadline").Base(err).WriteToLog(session.ExportIDToError(ctx))
	}

	defaultPort := net.Port(80)
	if strings.EqualFold(request.URL.Scheme, "https") {
		defaultPort = net.Port(443)
	}

	// 10.211.55.2:8080 要访问的目标主机
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}

	// 转成ip, port
	dest, err := http_proto.ParseHost(host, defaultPort)
	if err != nil {
		return newError("malformed proxy host: ", host).AtWarning().Base(err)
	}

	ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
		From:   conn.RemoteAddr(), // 对端地址
		To:     request.URL, // //10.211.55.2:8080
		Status: log.AccessAccepted, // accepted
		Reason: "",
	})

	if strings.EqualFold(request.Method, "CONNECT") {
		// dispatcher: 指向mux /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/mux/server.go
		return s.handleConnect(ctx, request, reader, conn, dest, dispatcher)
	}

	fmt.Println("IamNotConnect")

	keepAlive := (strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive")

	err = s.handlePlainHTTP(ctx, request, conn, dest, dispatcher)
	if err == errWaitAnother {
		if keepAlive {
			goto Start
		}
		err = nil
	}

	return err
}

// dispatcher: 指向mux /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/mux/server.go
func (s *Server) handleConnect(ctx context.Context, request *http.Request, reader *bufio.Reader, conn internet.Connection, dest net.Destination, dispatcher routing.Dispatcher) error {
	fmt.Println("in handleConnect1")

	// 返回内容
	_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		return newError("failed to write back OK response").Base(err)
	}

	/**
	Session{
			Timeouts: Timeout{
				//Align Handshake timeout with nginx client_header_timeout
				//So that this value will not indicate server identity
				Handshake:      time.Second * 60,
				ConnectionIdle: time.Second * 300,
				UplinkOnly:     time.Second * 1,
				DownlinkOnly:   time.Second * 1,
			},
			Stats: Stats{
				UserUplink:   false,
				UserDownlink: false,
			},
			Buffer: defaultBufferPolicy() Buffer{
								PerConnection: defaultBufferSize(-1),
							}
			/Users/demon/Desktop/work/gowork/src/v2ray.com/core/features/policy/policy.go
		}
	*/
	plcy := s.policy()
	ctx, cancel := context.WithCancel(ctx)

	// Timeouts.ConnectionIdle: time.Second * 300
	// time.AfterFunc, 超时会调用cancel
	timer := signal.CancelAfterInactivity(ctx, cancel, plcy.Timeouts.ConnectionIdle)

	ctx = policy.ContextWithBufferPolicy(ctx, plcy.Buffer)

	// /Users/demon/Desktop/work/gowork/src/v2ray.com/core/common/mux/server.go
	// 拿到的inbound

	/**
		1. 起个go:
				1. tcp连接目标
				2. 起两个go跑requestDone、OnSuccess
				3. 阻塞等待两个go
		2. 返回inbund
	*/
	link, err := dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return err
	}

	// 当前conn有数据送来
	if reader.Buffered() > 0 {
		fmt.Println("[ClientConn] reader.Buffered():", reader.Buffered())
		payload, err := buf.ReadFrom(io.LimitReader(reader, int64(reader.Buffered())))
		if err != nil {
			return err
		}
		if err := link.Writer.WriteMultiBuffer(payload); err != nil {
			return err
		}
		reader = nil
	}

	requestDone := func() error {
		// time.Second * 1,
		defer timer.SetTimeout(plcy.Timeouts.DownlinkOnly)
		/**
	 	&ReadVReader{
			Reader:  客户端conn的抽象io.Reader接口,
			rawConn: conn.(syscall.Conn)拿到原始Conn, 为什么要拿到原始的, 原始的是有fd的
			alloc: allocStrategy{
				current: 1,
			},
			mr: newMultiReader(),
		}

		buf.UpdateActivity(timer): 直接返回一个func
		*/
		// 可能也会阻塞
		fmt.Println("[ClientConn - request] plcy.Timeouts.DownlinkOnly:", plcy.Timeouts.DownlinkOnly)
		err := buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer), buf.SetGoType("[ClientConn - request]"))
		fmt.Println("[ClientConn - request] returned err:", err)
		return err
	}

	responseDone := func() error {
		defer timer.SetTimeout(plcy.Timeouts.UplinkOnly)

		v2writer := buf.NewWriter(conn)

		// 可能会阻塞
		fmt.Println("[ClientConn - response] plcy.Timeouts.UplinkOnly:", plcy.Timeouts.UplinkOnly)
		if err := buf.Copy(link.Reader, v2writer, buf.UpdateActivity(timer), buf.SetGoType("[ClientConn - response]")); err != nil {
			fmt.Println("[ClientConn - response] returned with err:", err)
			return err
		}
		fmt.Println("[ClientConn - response] returned")
		return nil
	}

	// 调试用
	// responseDone = func() error {
	// 	fmt.Println("[ClientConn] responseDone")
	// 	return nil
	// }
	
	// 传两个func, 再返回一个func
	var closeWriter = task.OnSuccess(requestDone, task.Close(link.Writer))

	if err := task.Run(ctx, closeWriter, responseDone); err != nil {
		fmt.Println("[Main] Run returned with err:", err)

		fmt.Println("[Main] I am gonna Interrupt Reader")
		common.Interrupt(link.Reader)

		fmt.Println("[Main] I am gonna Interrupt Writer")
		common.Interrupt(link.Writer)

		fmt.Println("[Main] goroutine ends with err:", err)
		return newError("connection ends").Base(err)
	}
	fmt.Println("[Main] goroutine ends")
	return nil
}

var errWaitAnother = newError("keep alive")

func (s *Server) handlePlainHTTP(ctx context.Context, request *http.Request, writer io.Writer, dest net.Destination, dispatcher routing.Dispatcher) error {
	if !s.config.AllowTransparent && request.URL.Host == "" {
		// RFC 2068 (HTTP/1.1) requires URL to be absolute URL in HTTP proxy.
		response := &http.Response{
			Status:        "Bad Request",
			StatusCode:    400,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header(make(map[string][]string)),
			Body:          nil,
			ContentLength: 0,
			Close:         true,
		}
		response.Header.Set("Proxy-Connection", "close")
		response.Header.Set("Connection", "close")
		return response.Write(writer)
	}

	if len(request.URL.Host) > 0 {
		request.Host = request.URL.Host
	}
	http_proto.RemoveHopByHopHeaders(request.Header)

	// Prevent UA from being set to golang's default ones
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", "")
	}

	content := &session.Content{
		Protocol: "http/1.1",
	}

	content.SetAttribute(":method", strings.ToUpper(request.Method))
	content.SetAttribute(":path", request.URL.Path)
	for key := range request.Header {
		value := request.Header.Get(key)
		content.SetAttribute(strings.ToLower(key), value)
	}

	ctx = session.ContextWithContent(ctx, content)

	link, err := dispatcher.Dispatch(ctx, dest)
	if err != nil {
		return err
	}

	// Plain HTTP request is not a stream. The request always finishes before response. Hense request has to be closed later.
	defer common.Close(link.Writer) // nolint: errcheck
	var result error = errWaitAnother

	requestDone := func() error {
		request.Header.Set("Connection", "close")

		requestWriter := buf.NewBufferedWriter(link.Writer)
		common.Must(requestWriter.SetBuffered(false))
		if err := request.Write(requestWriter); err != nil {
			return newError("failed to write whole request").Base(err).AtWarning()
		}
		return nil
	}

	responseDone := func() error {
		responseReader := bufio.NewReaderSize(&buf.BufferedReader{Reader: link.Reader}, buf.Size)
		response, err := http.ReadResponse(responseReader, request)
		if err == nil {
			http_proto.RemoveHopByHopHeaders(response.Header)
			if response.ContentLength >= 0 {
				response.Header.Set("Proxy-Connection", "keep-alive")
				response.Header.Set("Connection", "keep-alive")
				response.Header.Set("Keep-Alive", "timeout=4")
				response.Close = false
			} else {
				response.Close = true
				result = nil
			}
		} else {
			newError("failed to read response from ", request.Host).Base(err).AtWarning().WriteToLog(session.ExportIDToError(ctx))
			response = &http.Response{
				Status:        "Service Unavailable",
				StatusCode:    503,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header(make(map[string][]string)),
				Body:          nil,
				ContentLength: 0,
				Close:         true,
			}
			response.Header.Set("Connection", "close")
			response.Header.Set("Proxy-Connection", "close")
		}
		if err := response.Write(writer); err != nil {
			return newError("failed to write response").Base(err).AtWarning()
		}
		return nil
	}

	if err := task.Run(ctx, requestDone, responseDone); err != nil {
		common.Interrupt(link.Reader)
		common.Interrupt(link.Writer)
		return newError("connection ends").Base(err)
	}

	return result
}

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		fmt.Printf("  in /Users/demon/Desktop/work/gowork/src/v2ray.com/core/proxy/http/server.go, ctx:%+v  config:%+v\n", ctx, config)
		return NewServer(ctx, config.(*ServerConfig))
	}))
}
