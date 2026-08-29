package graceful

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/rotisserie/eris"
	"golang.org/x/sync/errgroup"
)

const (
	defaultHTTPPort     = "8080"
	defaultReadTimeout  = 60 * time.Second
	defaultWriteTimeout = 60 * time.Second
	// shutdownTimeout bounds in-flight requests once the group context is done.
	shutdownTimeout = 5 * time.Second
)

type server struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type ServerOpt func(*server)

func WithPort(port string) ServerOpt {
	return func(s *server) {
		s.Port = port
	}
}

func WithReadTimeout(timeout time.Duration) ServerOpt {
	return func(s *server) {
		s.ReadTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) ServerOpt {
	return func(s *server) {
		s.WriteTimeout = timeout
	}
}

func Server(router http.Handler, opts ...ServerOpt) Runner {
	cfg := &server{
		Port:         defaultHTTPPort,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context) error {
		group, groupCtx := errgroup.WithContext(ctx)

		server := &http.Server{
			Addr:           net.JoinHostPort("0.0.0.0", cfg.Port),
			Handler:        router,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		}

		group.Go(func() error {
			err := server.ListenAndServe()
			if eris.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})

		group.Go(func() error {
			<-groupCtx.Done()

			srvCtx, cancel := context.WithTimeout(context.WithoutCancel(groupCtx), shutdownTimeout)
			defer cancel()

			if err := server.Shutdown(srvCtx); err != nil {
				return err
			}

			return nil
		})

		return group.Wait()
	}
}
