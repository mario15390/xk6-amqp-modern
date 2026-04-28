// Package amqp provides a k6 extension for AMQP 0.9.1 (RabbitMQ) messaging.
//
// Architecture overview:
//
//   - RootModule: Singleton registered with k6 via init(). Acts as a factory
//     that creates one ModuleInstance per Virtual User (VU).
//
//   - ModuleInstance: Per-VU instance holding its own AMQP connection and
//     exposing all JavaScript-callable methods. This ensures thread safety
//     since each VU runs in its own goroutine and has isolated state.
//
//   - Event Loop Integration: Asynchronous operations (like Listen) use
//     vu.RegisterCallback() to safely schedule results back onto the k6
//     event loop, preventing the runtime panics that plagued the original
//     extension (see: github.com/grafana/xk6-amqp/issues/6 and #10).
//
// JavaScript API (all functions are named exports from 'k6/x/amqp'):
//
//	connect({ url })           — Open AMQP connection for this VU
//	close()                    — Close the VU's AMQP connection
//	publish({ ... })           — Publish a message to a queue/exchange
//	listen({ ... })            — Consume one message (returns Promise<string>)
//	declareQueue({ ... })      — Declare a queue
//	deleteQueue(name)          — Delete a queue
//	inspectQueue(name)         — Inspect queue metadata
//	bindQueue({ ... })         — Bind a queue to an exchange
//	unbindQueue({ ... })       — Unbind a queue from an exchange
//	purgeQueue(name)           — Purge all messages from a queue
//	declareExchange({ ... })   — Declare an exchange
//	deleteExchange(name)       — Delete an exchange
//	bindExchange({ ... })      — Bind an exchange to another exchange
//	unbindExchange({ ... })    — Unbind an exchange from another exchange
package amqp

import (
	"encoding/json"
	"fmt"
	"time"

	amqpDriver "github.com/rabbitmq/amqp091-go"
	"github.com/vmihailenco/msgpack/v5"
	"go.k6.io/k6/js/modules"
)

// version is the semantic version of this extension, used for identification.
const version = "v1.0.0"

// messagepack is the MIME type for MessagePack-encoded payloads.
// When PublishOptions.ContentType matches this, the JSON body is
// re-encoded as msgpack before sending.
const messagepack = "application/x-msgpack"

// ---------------------------------------------------------------------------
// Root Module — Factory for per-VU instances
// ---------------------------------------------------------------------------

// RootModule implements modules.Module and serves as the global singleton
// registered during init(). Its only job is to create a new ModuleInstance
// for each VU that imports 'k6/x/amqp'.
type RootModule struct{}

// NewModuleInstance is called by k6 once per VU. It creates a fresh
// ModuleInstance with its own connection state, ensuring complete
// isolation between VUs — no shared mutable state.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{
		vu:      vu,
		version: version,
	}

	// Build the named exports map. Each entry corresponds to a function
	// callable from JavaScript via: import { functionName } from 'k6/x/amqp';
	mi.exports = modules.Exports{
		Named: map[string]interface{}{
			// Connection lifecycle
			"version": version,
			"connect": mi.Connect,
			"close":   mi.Close,

			// Publish & consume
			"publish": mi.Publish,
			"listen":  mi.Listen,

			// Queue operations
			"declareQueue":  mi.DeclareQueue,
			"deleteQueue":   mi.DeleteQueue,
			"inspectQueue":  mi.InspectQueue,
			"bindQueue":     mi.BindQueue,
			"unbindQueue":   mi.UnbindQueue,
			"purgeQueue":    mi.PurgeQueue,

			// Exchange operations
			"declareExchange": mi.DeclareExchange,
			"deleteExchange":  mi.DeleteExchange,
			"bindExchange":    mi.BindExchange,
			"unbindExchange":  mi.UnbindExchange,
		},
	}
	return mi
}

// Compile-time interface compliance checks.
// These ensure RootModule and ModuleInstance satisfy the required k6 interfaces.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// init registers this extension with k6 under the 'k6/x/amqp' import path.
// This is the entry point that k6 calls when loading compiled extensions.
func init() {
	modules.Register("k6/x/amqp", new(RootModule))
}

// ---------------------------------------------------------------------------
// Module Instance — Per-VU state and methods
// ---------------------------------------------------------------------------

// ModuleInstance holds all per-VU state: the k6 VU reference, the AMQP
// connection, and the exported JavaScript API surface. Each VU gets its
// own instance via RootModule.NewModuleInstance().
type ModuleInstance struct {
	// vu is the k6 Virtual User context. It provides access to:
	//   - Runtime(): the Sobek JavaScript runtime for this VU
	//   - Context(): the Go context (cancelled when the VU stops)
	//   - RegisterCallback(): event loop integration for async ops
	vu modules.VU

	// conn is the AMQP connection for this VU. It is nil until Connect()
	// is called, and set back to nil after Close().
	conn *amqpDriver.Connection

	// version is the extension version string, exposed to JS.
	version string

	// exports holds the named functions exposed to JavaScript.
	exports modules.Exports
}

// Exports implements modules.Instance and returns the named exports
// that k6 will make available when a script does:
//
//	import { connect, publish, listen, ... } from 'k6/x/amqp';
func (mi *ModuleInstance) Exports() modules.Exports {
	return mi.exports
}

// ---------------------------------------------------------------------------
// Connection Lifecycle
// ---------------------------------------------------------------------------

// ConnectOptions holds the parameters for establishing an AMQP connection.
type ConnectOptions struct {
	// URL is the AMQP connection string (e.g. "amqp://guest:guest@localhost:5672/")
	URL string `js:"url"`
}

// Connect establishes an AMQP connection for this VU.
// It should be called once in the setup or init phase of a k6 script.
//
// JavaScript usage:
//
//	connect({ url: "amqp://guest:guest@localhost:5672/" });
func (mi *ModuleInstance) Connect(options ConnectOptions) error {
	if options.URL == "" {
		return fmt.Errorf("amqp: connection URL is required")
	}

	conn, err := amqpDriver.Dial(options.URL)
	if err != nil {
		return fmt.Errorf("amqp: failed to connect to %s: %w", options.URL, err)
	}

	// If there was a previous connection, close it gracefully
	if mi.conn != nil {
		_ = mi.conn.Close()
	}

	mi.conn = conn
	return nil
}

// Close gracefully shuts down the AMQP connection for this VU.
// After calling Close(), the VU must call Connect() again before
// performing any AMQP operations.
//
// JavaScript usage:
//
//	close();
func (mi *ModuleInstance) Close() error {
	if mi.conn == nil {
		return nil // Already closed or never opened — no-op
	}
	err := mi.conn.Close()
	mi.conn = nil
	return err
}

// getChannel is a helper that validates the connection is active and
// opens a new AMQP channel. Callers are responsible for closing the
// channel when done (typically via defer ch.Close()).
func (mi *ModuleInstance) getChannel() (*amqpDriver.Channel, error) {
	if mi.conn == nil {
		return nil, fmt.Errorf("amqp: not connected — call connect() first")
	}
	ch, err := mi.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("amqp: failed to open channel: %w", err)
	}
	return ch, nil
}

// ---------------------------------------------------------------------------
// Publish
// ---------------------------------------------------------------------------

// PublishOptions defines all parameters for publishing a message.
// Field names use `js:"snake_case"` tags to match the JavaScript convention
// from the original extension, maintaining backward compatibility.
type PublishOptions struct {
	QueueName     string            `js:"queue_name"`
	Body          string            `js:"body"`
	Headers       amqpDriver.Table  `js:"headers"`
	Exchange      string            `js:"exchange"`
	ContentType   string            `js:"content_type"`
	Mandatory     bool              `js:"mandatory"`
	Immediate     bool              `js:"immediate"`
	Persistent    bool              `js:"persistent"`
	CorrelationID string            `js:"correlation_id"`
	ReplyTo       string            `js:"reply_to"`
	Expiration    string            `js:"expiration"`
	MessageID     string            `js:"message_id"`
	Timestamp     int64             `js:"timestamp"`
	Type          string            `js:"type"`
	UserID        string            `js:"user_id"`
	AppID         string            `js:"app_id"`
}

// Publish sends a message to the specified queue or exchange.
// It opens a temporary channel, publishes the message, and closes
// the channel. This is safe for high-throughput scenarios as AMQP
// channels are lightweight.
//
// If ContentType is "application/x-msgpack", the Body (expected to be
// a JSON string) is re-encoded as MessagePack before sending.
//
// JavaScript usage:
//
//	publish({
//	    queue_name: "my-queue",
//	    body: "Hello, RabbitMQ!",
//	    content_type: "text/plain",
//	    exchange: "",          // optional, defaults to default exchange
//	    persistent: true,      // optional, marks message as persistent
//	    headers: { "x-foo": "bar" }, // optional
//	});
func (mi *ModuleInstance) Publish(options PublishOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	// Build the AMQP publishing message
	publishing := amqpDriver.Publishing{
		Headers:     options.Headers,
		ContentType: options.ContentType,
	}

	// Handle msgpack encoding: if the content type is msgpack,
	// parse the body as JSON first, then re-encode as msgpack bytes.
	if options.ContentType == messagepack {
		var jsonParsedBody interface{}

		if err = json.Unmarshal([]byte(options.Body), &jsonParsedBody); err != nil {
			return fmt.Errorf("amqp: failed to parse JSON body for msgpack encoding: %w", err)
		}

		publishing.Body, err = msgpack.Marshal(jsonParsedBody)
		if err != nil {
			return fmt.Errorf("amqp: failed to encode body as msgpack: %w", err)
		}
	} else {
		publishing.Body = []byte(options.Body)
	}

	// Set persistent delivery mode if requested
	if options.Persistent {
		publishing.DeliveryMode = amqpDriver.Persistent
	}

	// Set timestamp if provided (unix epoch seconds)
	if options.Timestamp != 0 {
		publishing.Timestamp = time.Unix(options.Timestamp, 0)
	}

	// Map remaining fields to the AMQP publishing struct
	publishing.CorrelationId = options.CorrelationID
	publishing.ReplyTo = options.ReplyTo
	publishing.Expiration = options.Expiration
	publishing.MessageId = options.MessageID
	publishing.Type = options.Type
	publishing.UserId = options.UserID
	publishing.AppId = options.AppID

	// Use the VU's context so publishing is cancelled if the VU stops
	return ch.PublishWithContext(
		mi.vu.Context(),
		options.Exchange,
		options.QueueName,
		options.Mandatory,
		options.Immediate,
		publishing,
	)
}

// ---------------------------------------------------------------------------
// Listen (Consume) — Event Loop Integration
// ---------------------------------------------------------------------------

// ListenOptions defines parameters for consuming a single message from a queue.
// The listen function returns a Promise that resolves with the message body.
type ListenOptions struct {
	QueueName string           `js:"queue_name"`
	Consumer  string           `js:"consumer"`
	AutoAck   bool             `js:"auto_ack"`
	Exclusive bool             `js:"exclusive"`
	NoLocal   bool             `js:"no_local"`
	NoWait    bool             `js:"no_wait"`
	Args      amqpDriver.Table `js:"args"`
}

// Listen consumes ONE message from the specified queue and returns it
// as a JavaScript Promise. This is the key architectural change from
// the original extension:
//
// OLD (broken): spawned a goroutine that called a JS callback directly,
// causing concurrent access panics on the JS runtime.
//
// NEW (safe): uses vu.RegisterCallback() to schedule the result back
// onto k6's event loop. The goroutine reads from the AMQP channel
// in the background, then uses the registered callback to safely
// resolve/reject the Promise on the main JS thread.
//
// Performance note: RegisterCallback + Promise adds minimal overhead
// (~microseconds) compared to the AMQP network round-trip (milliseconds).
// The goroutine is short-lived (waits for one message, then exits).
//
// JavaScript usage:
//
//	const message = await listen({
//	    queue_name: "my-queue",
//	    auto_ack: true,
//	});
//	console.log("Received:", message);
func (mi *ModuleInstance) Listen(options ListenOptions) interface{} {
	// Get the Sobek JS runtime for this VU to create a Promise
	rt := mi.vu.Runtime()

	// Create a new JS Promise. We get three things:
	//   - promise: the Promise object to return to JavaScript
	//   - resolve: function to call with the success value
	//   - reject:  function to call with the error value
	promise, resolve, reject := rt.NewPromise()

	// RegisterCallback tells k6's event loop: "I'm about to do async work,
	// please keep this VU alive until I call the returned function."
	// The returned 'callback' function is thread-safe and MUST be called
	// exactly once, or the VU will hang forever.
	callback := mi.vu.RegisterCallback()

	// Launch the blocking AMQP consume in a separate goroutine.
	// This goroutine will:
	//   1. Open a channel and start consuming
	//   2. Wait for one message (or context cancellation)
	//   3. Call the callback to schedule the result on the event loop
	go func() {
		ch, err := mi.getChannel()
		if err != nil {
			// Error opening channel — reject the Promise
			callback(func() error {
				reject(rt.ToValue(err.Error()))
				return nil
			})
			return
		}
		defer func() {
			_ = ch.Close()
		}()

		// Start consuming from the queue
		msgs, err := ch.Consume(
			options.QueueName,
			options.Consumer,
			options.AutoAck,
			options.Exclusive,
			options.NoLocal,
			options.NoWait,
			options.Args,
		)
		if err != nil {
			callback(func() error {
				reject(rt.ToValue(err.Error()))
				return nil
			})
			return
		}

		// Wait for either a message or context cancellation.
		// The select ensures we don't block forever if the VU is stopped.
		select {
		case msg, ok := <-msgs:
			if !ok {
				// Channel was closed (e.g., connection dropped)
				callback(func() error {
					reject(rt.ToValue("amqp: consumer channel closed"))
					return nil
				})
				return
			}
			// Successfully received a message — resolve the Promise
			// with the message body as a string.
			callback(func() error {
				resolve(rt.ToValue(string(msg.Body)))
				return nil
			})

		case <-mi.vu.Context().Done():
			// VU was stopped (test ended, timeout, etc.)
			callback(func() error {
				reject(rt.ToValue("amqp: VU context cancelled"))
				return nil
			})
		}
	}()

	// Return the Promise to JavaScript immediately.
	// The goroutine above will resolve/reject it asynchronously.
	return rt.ToValue(promise)
}
