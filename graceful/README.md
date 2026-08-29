# Graceful

Graceful is a lightweight Go package that provides utilities for managing graceful shutdown of your applications. It offers helper functions to handle OS signals (SIGINT, SIGTERM) and to execute multiple concurrent tasks with proper cancellation support.

## Features

- **Signal Handling:** Listens for termination signals (SIGINT and SIGTERM) to initiate a graceful shutdown.
- **Concurrent Execution:** Leverages Go's `errgroup` to run multiple tasks concurrently, ensuring clean shutdown and error propagation.

## Installation

Use `go get` to install the package:

```bash
go get github.com/LiquidCats/libraries/graceful
```

## Usage Example

Below is a simple example that demonstrates how to use the package to run a background task while listening for termination signals:

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LiquidCats/libraries/graceful"
)

func main() {
	ctx := context.Background()

	// Define a runner that performs a periodic task.
	taskRunner := func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				// Perform any cleanup here
				fmt.Println("TaskRunner shutting down...")
				return ctx.Err()
			default:
				fmt.Println("TaskRunner is running...")
				time.Sleep(2 * time.Second)
			}
		}
	}

	// Wait for either the task to complete or a termination signal.
	err := graceful.WaitContext(ctx, taskRunner, graceful.Signals)
	if err != nil {
		fmt.Println("Exited with error:", err)
	}

	fmt.Println("Exited gracefully.")
}
```

## How It Works

- **Signals Function:**
    - Creates a channel to receive OS signals (SIGINT, SIGTERM).
    - Returns when one of the signals is received, initiating shutdown.

- **WaitContext Function:**
    - Accepts a context and one or more runner functions (each with the signature `func(context.Context) error`).
    - Runs each runner concurrently using an `errgroup`.
    - Cancels the context if any runner returns an error or if a termination signal is received.

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests for bug fixes, improvements, or new features.

## License

Distributed under the MIT License. See `LICENSE` for more information.
