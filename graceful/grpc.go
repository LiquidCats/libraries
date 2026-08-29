package graceful

import (
	"context"
	"net"
	"time"

	"github.com/rotisserie/eris"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	defaultGRPCPort          = "50051"
	defaultConnectionTimeout = 120 * time.Second
)

type grpcSrv struct {
	port              string
	connectionTimeout time.Duration
}

type GRPCAttacher interface {
	AttachToGRPC(grpc.ServiceRegistrar)
}

type GRPCOpt func(*grpcSrv)

func WithGRPCPort(port string) GRPCOpt {
	return func(g *grpcSrv) {
		g.port = port
	}
}

func WithConnectionTimeout(timeout time.Duration) GRPCOpt {
	return func(g *grpcSrv) {
		g.connectionTimeout = timeout
	}
}

func GRPCRunner(attacher GRPCAttacher, opts ...GRPCOpt) Runner {
	cfg := &grpcSrv{
		port:              defaultGRPCPort,
		connectionTimeout: defaultConnectionTimeout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	grpcServer := grpc.NewServer(grpc.ConnectionTimeout(cfg.connectionTimeout))

	attacher.AttachToGRPC(grpcServer)

	return func(ctx context.Context) error {
		var lc net.ListenConfig

		lis, err := lc.Listen(ctx, "tcp", net.JoinHostPort("", cfg.port))
		if err != nil {
			return eris.Wrap(err, "failed to listen")
		}

		group, ctx := errgroup.WithContext(ctx)

		group.Go(func() error {
			return grpcServer.Serve(lis)
		})

		group.Go(func() error {
			<-ctx.Done()

			grpcServer.GracefulStop()

			return nil
		})

		return group.Wait()
	}
}
