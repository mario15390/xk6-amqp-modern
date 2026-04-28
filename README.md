# xk6-amqp-modern

A modern k6 extension for publishing and consuming messages via AMQP 0.9.1 (RabbitMQ).

This is a **complete rewrite** of [grafana/xk6-amqp](https://github.com/grafana/xk6-amqp), modernized to work with the latest k6 versions using the proper Module/Instance API and event loop integration.

> ⚠️ This project uses [AMQP 0.9.1](https://www.rabbitmq.com/tutorials/amqp-concepts.html), not [AMQP 1.0](http://docs.oasis-open.org/amqp/core/v1.0/os/amqp-core-overview-v1.0-os.html).

---

## Table of Contents

- [Quick Start](#quick-start)
- [Build](#build)
- [JavaScript API](#javascript-api)
- [Architecture & Design](#architecture--design)
- [Project Structure](#project-structure)
- [Testing](#testing)
- [Docker](#docker)
- [Examples](#examples)

---

## Quick Start

```bash
# Build k6 with extension using Docker
docker compose build k6-build

# Start RabbitMQ
docker compose up -d rabbitmq

# Run a test
docker compose run --rm k6-integration
```

## Build

### For Windows
```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.25-alpine sh -c 'apk add --no-cache git && go install go.k6.io/xk6/cmd/xk6@latest && GOOS=windows GOARCH=amd64 $(go env GOPATH)/bin/xk6 build --with github.com/mario15390/xk6-amqp-modern=. --output k6.exe'
```
### For Linux
```bash
docker run --rm -v "${PWD}:/src" -w /src golang:1.25-alpine sh -c 'apk add --no-cache git && go install go.k6.io/xk6/cmd/xk6@latest && GOOS=linux GOARCH=amd64 $(go env GOPATH)/bin/xk6 build --with github.com/mario15390/xk6-amqp-modern=. --output k6-linux'
```


### Using Docker (Recommended)

```bash
docker compose build k6-build
```

This produces a Docker image `k6-amqp:local` with the custom k6 binary.

### From Source

Prerequisites: Go 1.23+, Git

```bash
# Install xk6
go install go.k6.io/xk6/cmd/xk6@latest

# Build k6 binary with this extension
xk6 build --with github.com/mario15390/xk6-amqp-modern=.
```

---

## JavaScript API

All functions are **named exports** from `k6/x/amqp`:

```javascript
import {
  connect, close,
  publish, listen,
  declareQueue, deleteQueue, inspectQueue, bindQueue, unbindQueue, purgeQueue,
  declareExchange, deleteExchange, bindExchange, unbindExchange,
} from 'k6/x/amqp';
```

### Connection

| Function | Description |
|---|---|
| `connect({ url })` | Open an AMQP connection for this VU |
| `close()` | Close the AMQP connection |

### Publishing & Consuming

| Function | Description |
|---|---|
| `publish({ queue_name, body, content_type, ... })` | Publish a message |
| `await listen({ queue_name, auto_ack, ... })` | Consume one message (async, returns Promise) |

### Queue Operations

| Function | Description |
|---|---|
| `declareQueue({ name, durable, ... })` | Declare/create a queue |
| `deleteQueue(name)` | Delete a queue |
| `inspectQueue(name)` | Get queue metadata (messages, consumers) |
| `bindQueue({ queue_name, exchange_name, routing_key })` | Bind queue to exchange |
| `unbindQueue({ queue_name, exchange_name, routing_key })` | Unbind queue from exchange |
| `purgeQueue(name)` | Remove all messages from a queue |

### Exchange Operations

| Function | Description |
|---|---|
| `declareExchange({ name, kind, durable, ... })` | Declare/create an exchange |
| `deleteExchange(name)` | Delete an exchange |
| `bindExchange({ source_exchange_name, destination_exchange_name, routing_key })` | Bind exchanges |
| `unbindExchange({ source_exchange_name, destination_exchange_name, routing_key })` | Unbind exchanges |

### Example

```javascript
import { connect, publish, listen, declareQueue, deleteQueue, close } from 'k6/x/amqp';
import { check } from 'k6';

export default async function () {
  connect({ url: "amqp://guest:guest@localhost:5672/" });

  const q = declareQueue({ name: "my-queue" });
  console.log(`Queue ${q.name} ready (${q.messages} messages)`);

  publish({
    queue_name: "my-queue",
    body: "Hello from k6!",
    content_type: "text/plain",
  });

  const msg = await listen({
    queue_name: "my-queue",
    auto_ack: true,
  });

  check(msg, {
    'message received': (m) => m === "Hello from k6!",
  });

  deleteQueue("my-queue");
  close();
}
```

---

## Architecture & Design

### Why was the rewrite needed?

The original `xk6-amqp` extension had three critical issues:

1. **Legacy Module API**: Used `modules.Register("k6/x/amqp", &struct{})` which registers a single shared instance. Modern k6 requires `modules.Module` + `modules.Instance` interfaces for per-VU isolation.

2. **Shared Mutable State**: All VUs shared a global `map[int]*Connection` via pointers, causing race conditions with multiple VUs.

3. **Unsafe Async**: The `Listen()` function spawned goroutines that directly called JavaScript callbacks, bypassing k6's event loop and causing panics from concurrent access to the JS runtime.

### Solution Applied

```
┌─────────────────────────────────────────────────────────┐
│                    k6 Engine                            │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │   VU 1   │  │   VU 2   │  │   VU N   │              │
│  │          │  │          │  │          │              │
│  │ ┌──────┐ │  │ ┌──────┐ │  │ ┌──────┐ │              │
│  │ │Module│ │  │ │Module│ │  │ │Module│ │  ◄── Each VU  │
│  │ │Inst. │ │  │ │Inst. │ │  │ │Inst. │ │     gets its  │
│  │ │      │ │  │ │      │ │  │ │      │ │     own inst. │
│  │ │ conn │ │  │ │ conn │ │  │ │ conn │ │              │
│  │ └──┬───┘ │  │ └──┬───┘ │  │ └──┬───┘ │              │
│  └────┼─────┘  └────┼─────┘  └────┼─────┘              │
│       │             │             │                     │
└───────┼─────────────┼─────────────┼─────────────────────┘
        │             │             │
        ▼             ▼             ▼
   ┌─────────────────────────────────────┐
   │          RabbitMQ Server            │
   │       (AMQP 0.9.1 Broker)          │
   └─────────────────────────────────────┘
```

#### Key Design Decisions

| Decision | Rationale |
|---|---|
| **Per-VU connections** | Eliminates shared state and race conditions. Each VU's `ModuleInstance` has its own `*amqpDriver.Connection`. |
| **`RootModule` factory pattern** | `NewModuleInstance(vu)` is called once per VU, providing the `modules.VU` reference for event loop access. |
| **Promise-based `listen()`** | Uses `vu.RegisterCallback()` + `rt.NewPromise()` to safely return async results to JavaScript via k6's event loop. |
| **Named exports** (no sub-modules) | Original had separate `k6/x/amqp/queue` and `k6/x/amqp/exchange` modules with shared state. Consolidated into single `k6/x/amqp` module. |
| **`js:"snake_case"` struct tags** | Maintains JavaScript naming convention compatibility with the original extension's API. |

#### Event Loop Integration (Listen)

The `listen()` function is the most architecturally important change:

```
JavaScript Thread                     Background Goroutine
─────────────────                     ────────────────────
1. Call listen()
2. Create Promise
3. RegisterCallback() ──────────────► 4. Open AMQP channel
   (tells k6: "async                  5. Start consuming
    work in progress")                 6. Wait for message...
                                       7. Message arrives!
   return Promise ◄────────────────── 8. callback(func() {
                                          resolve(message)
                                       })
9. await resolves with message
10. Continue script execution
```

`RegisterCallback()` is **critical**: it tells k6's event loop to keep the VU alive while the goroutine runs. The callback function is the only safe way to interact with the JS runtime from a goroutine.

---

## Project Structure

```
xk6-amqp-modern/
├── amqp.go                 # Core: RootModule, ModuleInstance, Connect, Publish, Listen
├── amqp_test.go            # Unit tests for the core module
├── queues.go               # Queue operations (DeclareQueue, DeleteQueue, etc.)
├── exchanges.go            # Exchange operations (DeclareExchange, BindExchange, etc.)
├── go.mod                  # Go module definition and dependencies
├── go.sum                  # Go dependency checksums
├── Makefile                # Build, test, and Docker targets
├── Dockerfile              # Multi-stage build for k6 binary
├── Dockerfile.test         # Container for running Go tests
├── docker-compose.yml      # Full stack: RabbitMQ + build + tests
├── .golangci.yml           # Go linter configuration
├── integration/            # Integration tests
│   ├── integration_test.go # Go tests requiring RabbitMQ (build tag: integration)
│   └── scripts/            # k6 test scripts
│       ├── test_publish_listen.js   # Basic publish/listen test
│       ├── test_queue_ops.js        # Queue operations test
│       ├── test_exchange_ops.js     # Exchange operations test
│       ├── test_multi_vu.js         # Multi-VU performance test
│       └── test_publisher_only.js   # Multiple sources publisher test
├── examples/               # Usage examples
│   ├── basic.js            # Simple publish and listen
│   ├── queue-operations.js # Queue lifecycle demo
│   └── exchange-routing.js # Exchange routing demo
└── README.md               # This file
```

### File Responsibilities

| File | What it does |
|---|---|
| `amqp.go` | Defines `RootModule` (factory) and `ModuleInstance` (per-VU state). Contains `Connect()`, `Close()`, `Publish()`, `Listen()`, and the `init()` registration. |
| `queues.go` | All queue-related methods on `ModuleInstance`: declare, delete, inspect, bind, unbind, purge. Each method opens a temporary AMQP channel. |
| `exchanges.go` | All exchange-related methods on `ModuleInstance`: declare, delete, bind, unbind. Same channel pattern as queues. |
| `amqp_test.go` | Unit tests that run WITHOUT RabbitMQ. Tests export names, error handling, option defaults. |
| `integration/integration_test.go` | Tests that require a running RabbitMQ. Guarded by `//go:build integration` build tag. |

---

## Build k6 with Extension

### Option 1: Compiling for Windows / Linux using Docker (No local Go required)

If you don't want to install Go locally, you can use Docker to cross-compile the binary. 

> [!WARNING]
> **Windows Users**: Run these commands in **PowerShell** or **Command Prompt**, NOT Git Bash. Git Bash alters paths in volume mounts and will cause a `working directory invalid` error.

**Compile `k6.exe` for Windows (x64)**
```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.25-alpine sh -c 'apk add --no-cache git && go install go.k6.io/xk6/cmd/xk6@latest && GOOS=windows GOARCH=amd64 $(go env GOPATH)/bin/xk6 build --with github.com/mario15390/xk6-amqp-modern=. --output k6.exe'
```

**Compile `k6` for Linux (Ubuntu/Debian x64)**
```bash
docker run --rm -v "${PWD}:/src" -w /src golang:1.25-alpine sh -c 'apk add --no-cache git && go install go.k6.io/xk6/cmd/xk6@latest && GOOS=linux GOARCH=amd64 $(go env GOPATH)/bin/xk6 build --with github.com/mario15390/xk6-amqp-modern=. --output k6-linux'
```

Once the command finishes, you will find the binary (`k6.exe` or `k6-linux`) in your project folder. You can then run your test scripts directly:
```bash
# Windows
.\k6.exe run examples\basic.js

# Linux
./k6-linux run examples/basic.js
```

### Option 2: Local Compilation (Requires Go)

If you have Go installed on your local machine, you can build the custom `k6` binary directly:
```bash
go install go.k6.io/xk6/cmd/xk6@latest
xk6 build --with github.com/mario15390/xk6-amqp-modern=.
./k6 run examples/basic.js
```

---

## Testing

### Run All Tests with Docker

```bash
# Run everything (unit + integration + k6)
make docker-test-all

# Or individually:
make docker-unit-test          # Unit tests only
make docker-integration-test   # Integration tests (starts RabbitMQ)
make docker-k6-test            # k6 script tests
```

### Run Unit Tests Locally

```bash
go test -v -race -cover ./...
```

### Run Integration Tests Locally

```bash
# Start RabbitMQ first
docker compose up -d rabbitmq

# Wait for it to be ready, then run integration tests
go test -v -tags=integration -timeout=120s ./integration/...
```

### Performance Testing

The `docker-compose.yml` file is configured to map the local `./integration/scripts` folder as a volume (`/scripts`) inside the container. This means you can edit the `.js` files locally and the container will see the changes instantly, without needing to rebuild the Docker image!

```bash
# Run the default performance test (5 VUs, 20 iterations)
make docker-performance

# Custom: run test_multi_vu.js with 50 VUs for 1 minute
docker compose run --rm k6-performance run /scripts/test_multi_vu.js --vus 50 --duration 1m

# Simulate multiple sources publishing (NO consuming)
docker compose run --rm k6-performance run /scripts/test_publisher_only.js
```

> [!NOTE]
> Open `integration/scripts/test_publisher_only.js` to see predefined commented configurations (Light, Moderate, Heavy loads). Simply uncomment the one you want to use and run the command above.

The performance tests report custom metrics:
- `amqp_messages_published` — Counter of published messages
- `amqp_messages_received` — Counter of consumed messages
- `amqp_publish_duration_ms` — Trend of publish latencies
- `amqp_listen_duration_ms` — Trend of listen latencies

---

## Large Scale Load Testing Strategy

When testing with massive concurrency (e.g., 5,000+ VUs), you are no longer just testing the extension's code; you are pushing the limits of the OS network stack, Docker's virtual network, and RabbitMQ's connection handlers. 

If you start 5,000 VUs instantly, you will create a **Connection Storm**. 5,000 TCP sockets attempting to open in the exact same millisecond will likely result in `read tcp ... i/o timeout` errors as the host or container network queues overflow.

### 1. RabbitMQ Docker Tuning (Implemented)
To handle high connection density, the provided `docker-compose.yml` tunes RabbitMQ by:
- Setting `ulimits: nofile: soft: 65536 / hard: 65536` to allow 65k concurrent TCP connections.
- Adding `RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=+P 1048576` to increase the Erlang process limits.

### 2. k6 Scenarios
Never use `options = { vus: 5000, duration: '1m' }` for large scale tests!
Instead, use the k6 `scenarios` API with the `ramping-vus` executor to gradually increase load. This allows RabbitMQ to process connection handshakes smoothly.

Open `integration/scripts/test_publisher_only.js` to find predefined load profiles:
- **Smoke Test**: Validates functionality (1 VU).
- **Load Test**: Normal sustained load (500 VUs constant).
- **Stress Test**: Pushes limits to find breaking points (ramping to 5000 VUs).
- **Spike Test**: Sudden massive surges (baseline 100 -> jump to 4000).
- **Soak Test**: Sustained load over hours to detect memory leaks.

### 3. Interpreting Results
When running large tests, pay attention to these metrics:
1. **`amqp_publish_duration_ms`**: If this spikes severely, RabbitMQ is struggling to process the messages (CPU/Memory bound). Check if `vm_memory_high_watermark` is triggered in the RabbitMQ management UI.
2. **`i/o timeout` errors**: If you see these during a gradual ramp-up, the network stack is saturated. 
3. **CPU vs Network**: Running k6 and RabbitMQ on the same machine under heavy load will cause them to compete for CPU. 

> [!WARNING]
> **Docker Desktop Limits**: Docker on Windows (WSL2) and Mac uses a lightweight virtual machine. The virtualized network proxy between the host and the VM becomes a severe bottleneck long before RabbitMQ or k6 actually fails. **For accurate large-scale QA/Preprod testing, run k6 and RabbitMQ natively on separate Linux servers or dedicated cloud instances.**

---

## Docker

### Services

| Service | Purpose |
|---|---|
| `rabbitmq` | RabbitMQ 3.x with Management Plugin |
| `k6-build` | Builds the custom k6 binary |
| `unit-tests` | Runs Go unit tests |
| `integration-tests` | Runs Go integration tests (needs RabbitMQ) |
| `k6-integration` | Runs k6 integration script |
| `k6-performance` | Runs k6 performance test |

### RabbitMQ Management UI

After starting RabbitMQ:

```bash
docker compose up -d rabbitmq
```

Access the management UI at [http://localhost:15672](http://localhost:15672) (login: `guest` / `guest`).

---

## Examples

See the `examples/` directory for usage demos, and `integration/scripts/` for test scripts.

## License

See [LICENSE](LICENSE) file.
