package graceful_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/LiquidCats/libraries/graceful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getFreePort returns an unused TCP port as a string.
func getFreePort() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("failed to get free port: %v", err))
	}
	defer func() {
		_ = l.Close()
	}()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

// simplePingHandler returns "pong" for any request.
func simplePingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}

// Test that the server starts, listens on the specified port, and responds to requests.
func TestServerStartsAndResponds(t *testing.T) {
	port := getFreePort()
	router := http.HandlerFunc(simplePingHandler)
	runner := graceful.Server(router, graceful.WithPort(port))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		_ = runner(ctx)
	})

	// Wait for server to start. Retry a few times.
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%s/ping", port)
	var resp *http.Response
	for range 5 {
		resp, _ = client.Get(url)
		if resp != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.NotNil(t, resp, "server did not start")
	defer func() {
		_ = resp.Body.Close()
	}()

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "pong", buf.String())

	// Shutdown the server
	cancel()
	wg.Wait()
}

func TestServerInvalidPortReturnsError(t *testing.T) {
	router := http.HandlerFunc(simplePingHandler)
	runner := graceful.Server(router, graceful.WithPort("invalid-port"))

	err := runner(context.Background())
	assert.Error(t, err)
}
