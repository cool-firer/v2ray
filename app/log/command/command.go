// +build !confonly

package command

//go:generate go run v2ray.com/core/common/errors/errorgen

import (
	"context"

	"fmt"
	grpc "google.golang.org/grpc"

	"v2ray.com/core"
	"v2ray.com/core/app/log"
	"v2ray.com/core/common"
)

type LoggerServer struct {
	V *core.Instance
}

// RestartLogger implements LoggerService.
func (s *LoggerServer) RestartLogger(ctx context.Context, request *RestartLoggerRequest) (*RestartLoggerResponse, error) {
	logger := s.V.GetFeature((*log.Instance)(nil))
	if logger == nil {
		return nil, newError("unable to get logger instance")
	}
	if err := logger.Close(); err != nil {
		return nil, newError("failed to close logger").Base(err)
	}
	if err := logger.Start(); err != nil {
		return nil, newError("failed to start logger").Base(err)
	}
	return &RestartLoggerResponse{}, nil
}

func (s *LoggerServer) mustEmbedUnimplementedLoggerServiceServer() {}

type service struct {
	v *core.Instance
}

func (s *service) Register(server *grpc.Server) {
	RegisterLoggerServiceServer(server, &LoggerServer{
		V: s.v,
	})
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		fmt.Printf("in core.app.log.common log.go, ctx:%+v  config:%+v\n", ctx, cfg)
		s := core.MustFromContext(ctx)
		return &service{v: s}, nil
	}))
}
