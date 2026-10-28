package graceful

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/rotisserie/eris"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultGRPCPort          = "50051"
	defaultConnectionTimeout = 120 * time.Second
)

type grpcSrv struct {
	port              string
	connectionTimeout time.Duration
	unaryInterceptors []grpc.UnaryServerInterceptor
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

func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) GRPCOpt {
	return func(g *grpcSrv) {
		g.unaryInterceptors = append(g.unaryInterceptors, interceptor)
	}
}

type errLogger[F any] interface {
	Error(msg string, args ...F)
}

func WithPanicReocvery[F any](l errLogger[F]) GRPCOpt {
	return func(g *grpcSrv) {
		g.unaryInterceptors = append(g.unaryInterceptors, func(
			ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
		) (resp any, err error) {
			defer func() {
				if r := recover(); r != nil {
					l.Error(fmt.Sprintf("recover after panic: %v", r))

					resp, err = nil, status.Error(codes.Internal, "internal error")
				}
			}()

			return handler(ctx, req)
		})
	}
}

func WithConnectionTimeout(timeout time.Duration) GRPCOpt {
	return func(g *grpcSrv) {
		g.connectionTimeout = timeout
	}
}

type infoLogger[F any] interface {
	Info(msg string, args ...F)
}

func WithRequestLogger[F any](l infoLogger[F]) GRPCOpt {
	return func(g *grpcSrv) {
		g.unaryInterceptors = append(g.unaryInterceptors, func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (any, error) {
			now := time.Now()
			defer func() {
				l.Info(fmt.Sprintf("incoming request: %s, duration: %v", info.FullMethod, time.Since(now)))
			}()

			return handler(ctx, req)
		})
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

	grpcServer := grpc.NewServer(
		grpc.ConnectionTimeout(cfg.connectionTimeout),
		grpc.ChainUnaryInterceptor(cfg.unaryInterceptors...),
	)

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
