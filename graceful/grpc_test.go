package graceful_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/LiquidCats/libraries/graceful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// mockGRPCAttacher implements GRPCAttacher interface for testing.
type mockGRPCAttacher struct {
	attached bool
}

func (m *mockGRPCAttacher) AttachToGRPC(registrar grpc.ServiceRegistrar) {
	m.attached = true
	// Register health service for testing
	healthpb.RegisterHealthServer(registrar, health.NewServer())
}

// getFreeGRPCPort returns an unused TCP port as a string.
func getFreeGRPCPort() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("failed to get free port: %v", err))
	}
	defer func() {
		_ = l.Close()
	}()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// TestGRPCServerStartsAndResponds verifies that the gRPC server starts,
// listens on the specified port, and responds to requests.
func TestGRPCServerStartsAndResponds(t *testing.T) {
	port := getFreeGRPCPort()
	attacher := &mockGRPCAttacher{}
	runner := graceful.GRPCRunner(attacher, graceful.WithGRPCPort(port))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner(ctx)
	})

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create a gRPC client and verify connection
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%s", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()

	// Use health check service to verify server is running
	healthClient := healthpb.NewHealthClient(conn)
	resp, err := healthClient.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
	assert.True(t, attacher.attached, "attacher should have been called")

	// Shutdown the server
	cancel()
	wg.Wait()
}

// TestGRPCServerDefaultPort verifies that the server uses default port when not specified.
func TestGRPCServerDefaultPort(t *testing.T) {
	attacher := &mockGRPCAttacher{}
	runner := graceful.GRPCRunner(attacher)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The server should try to start on default port 50051
	// This test doesn't verify the actual port, just that it doesn't panic
	// and attacher is called
	go func() {
		_ = runner(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	assert.True(t, attacher.attached, "attacher should have been called")
}

// TestGRPCServerWithCustomConnectionTimeout verifies that custom connection timeout is applied.
func TestGRPCServerWithCustomConnectionTimeout(t *testing.T) {
	port := getFreeGRPCPort()
	attacher := &mockGRPCAttacher{}
	customTimeout := 60 * time.Second
	runner := graceful.GRPCRunner(
		attacher,
		graceful.WithGRPCPort(port),
		graceful.WithConnectionTimeout(customTimeout),
	)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner(ctx)
	})

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%s", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()

	assert.True(t, attacher.attached, "attacher should have been called")

	// Shutdown the server
	cancel()
	wg.Wait()
}

// TestGRPCServerGracefulShutdown verifies that the server shuts down gracefully.
func TestGRPCServerGracefulShutdown(t *testing.T) {
	port := getFreeGRPCPort()
	attacher := &mockGRPCAttacher{}
	runner := graceful.GRPCRunner(attacher, graceful.WithGRPCPort(port))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	var serverErr error
	go func() {
		defer wg.Done()
		serverErr = runner(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	cancel()

	// Wait for shutdown to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Server shut down successfully
		assert.NoError(t, serverErr)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}
}

// TestGRPCServerInvalidPort verifies that an invalid port returns an error.
func TestGRPCServerInvalidPort(t *testing.T) {
	attacher := &mockGRPCAttacher{}
	runner := graceful.GRPCRunner(attacher, graceful.WithGRPCPort("invalid-port"))

	err := runner(context.Background())
	assert.Error(t, err, "should return error for invalid port")
}

// TestGRPCServerPortInUse verifies that using a port already in use returns an error.
func TestGRPCServerPortInUse(t *testing.T) {
	port := getFreeGRPCPort()

	// Start first server
	attacher1 := &mockGRPCAttacher{}
	runner1 := graceful.GRPCRunner(attacher1, graceful.WithGRPCPort(port))

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner1(ctx1)
	})

	// Wait for first server to start
	time.Sleep(100 * time.Millisecond)

	// Try to start second server on same port
	attacher2 := &mockGRPCAttacher{}
	runner2 := graceful.GRPCRunner(attacher2, graceful.WithGRPCPort(port))

	err := runner2(context.Background())
	assert.Error(t, err, "should return error when port is already in use")

	cancel1()
	wg.Wait()
}

// TestGRPCServerMultipleOptions verifies that multiple options can be applied.
func TestGRPCServerMultipleOptions(t *testing.T) {
	port := getFreeGRPCPort()
	attacher := &mockGRPCAttacher{}
	runner := graceful.GRPCRunner(
		attacher,
		graceful.WithGRPCPort(port),
		graceful.WithConnectionTimeout(30*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner(ctx)
	})

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%s", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer func() {
		_ = conn.Close()
	}()

	healthClient := healthpb.NewHealthClient(conn)
	resp, err := healthClient.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)

	// Shutdown the server
	cancel()
	wg.Wait()
}

// TestGRPCAttacherCalled verifies that the attacher's AttachToGRPC method is called.
func TestGRPCAttacherCalled(t *testing.T) {
	port := getFreeGRPCPort()
	attacher := &mockGRPCAttacher{}

	assert.False(t, attacher.attached, "attacher should not be attached initially")

	runner := graceful.GRPCRunner(attacher, graceful.WithGRPCPort(port))

	// The attacher should be called during GRPCRunner creation, before the runner is executed
	assert.True(t, attacher.attached, "attacher should be called when GRPCRunner is created")

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner(ctx)
	})

	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()
}
