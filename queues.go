// queues.go — Queue operations for the xk6-amqp extension.
//
// All queue methods are defined on ModuleInstance, meaning each VU
// uses its own AMQP connection. There are no shared resources.
//
// Queue operations are synchronous (they block the VU goroutine
// until the AMQP server responds). This is fine because:
//   - Queue operations are fast (metadata only, no message payloads)
//   - They are typically called in setup/init, not in the hot loop
//   - k6 VUs are independent goroutines — blocking one doesn't affect others
package amqp

import (
	"fmt"

	amqpDriver "github.com/rabbitmq/amqp091-go"
)

// ---------------------------------------------------------------------------
// Queue Option Types
// ---------------------------------------------------------------------------

// DeclareQueueOptions provides parameters when declaring (creating) a queue.
// All fields are optional except Name.
type DeclareQueueOptions struct {
	// Name is the queue name. If empty, the server generates a unique name.
	Name string `js:"name"`

	// Durable queues survive broker restarts. Non-durable queues are deleted
	// when the broker shuts down.
	Durable bool `js:"durable"`

	// DeleteWhenUnused causes the queue to be deleted when the last
	// consumer unsubscribes.
	DeleteWhenUnused bool `js:"delete_when_unused"`

	// Exclusive queues are only accessible by the connection that
	// declared them and are deleted when that connection closes.
	Exclusive bool `js:"exclusive"`

	// NoWait: if true, the server will not respond to the declare.
	// The client should not wait for a response.
	NoWait bool `js:"no_wait"`

	// Args are optional arguments for the queue (e.g., x-message-ttl).
	Args amqpDriver.Table `js:"args"`
}

// QueueBindOptions provides parameters when binding a queue to an exchange.
// Binding tells the exchange to route messages to this queue based on
// the routing key.
type QueueBindOptions struct {
	// QueueName is the name of the queue to bind.
	QueueName string `js:"queue_name"`

	// ExchangeName is the name of the exchange to bind to.
	ExchangeName string `js:"exchange_name"`

	// RoutingKey is the routing pattern for the binding.
	RoutingKey string `js:"routing_key"`

	// NoWait: if true, do not wait for server confirmation.
	NoWait bool `js:"no_wait"`

	// Args are optional arguments for the binding.
	Args amqpDriver.Table `js:"args"`
}

// QueueUnbindOptions provides parameters when removing a queue binding.
type QueueUnbindOptions struct {
	// QueueName is the name of the queue to unbind.
	QueueName string `js:"queue_name"`

	// ExchangeName is the name of the exchange to unbind from.
	ExchangeName string `js:"exchange_name"`

	// RoutingKey is the routing key of the binding to remove.
	RoutingKey string `js:"routing_key"`

	// Args must match the args used when the binding was created.
	Args amqpDriver.Table `js:"args"`
}

// ---------------------------------------------------------------------------
// Queue Methods (on ModuleInstance)
// ---------------------------------------------------------------------------

// DeclareQueue creates or verifies a queue on the AMQP server.
// If the queue already exists with identical properties, this is a no-op.
// If it exists with different properties, the server will return an error.
//
// Returns a map with queue metadata:
//
//	{ name: "my-queue", messages: 0, consumers: 0 }
//
// JavaScript usage:
//
//	const q = declareQueue({ name: "my-queue", durable: true });
//	console.log(`Queue ${q.name} has ${q.messages} messages`);
func (mi *ModuleInstance) DeclareQueue(options DeclareQueueOptions) (map[string]interface{}, error) {
	ch, err := mi.getChannel()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = ch.Close()
	}()

	q, err := ch.QueueDeclare(
		options.Name,
		options.Durable,
		options.DeleteWhenUnused,
		options.Exclusive,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		return nil, fmt.Errorf("amqp: failed to declare queue %q: %w", options.Name, err)
	}

	// Return queue metadata as a JS-friendly map
	return map[string]interface{}{
		"name":      q.Name,
		"messages":  q.Messages,
		"consumers": q.Consumers,
	}, nil
}

// DeleteQueue removes a queue from the AMQP server.
// Any pending messages in the queue are discarded.
//
// JavaScript usage:
//
//	deleteQueue("my-queue");
func (mi *ModuleInstance) DeleteQueue(name string) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	_, err = ch.QueueDelete(
		name,
		false, // ifUnused — delete even if consumers are attached
		false, // ifEmpty — delete even if messages are pending
		false, // noWait
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to delete queue %q: %w", name, err)
	}
	return nil
}

// InspectQueue returns metadata about a queue without modifying it.
// This is useful for checking message counts and consumer counts.
//
// Returns a map:
//
//	{ name: "my-queue", messages: 42, consumers: 3 }
//
// JavaScript usage:
//
//	const info = inspectQueue("my-queue");
//	console.log(`${info.messages} messages waiting`);
func (mi *ModuleInstance) InspectQueue(name string) (map[string]interface{}, error) {
	ch, err := mi.getChannel()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = ch.Close()
	}()

	q, err := ch.QueueInspect(name)
	if err != nil {
		return nil, fmt.Errorf("amqp: failed to inspect queue %q: %w", name, err)
	}

	return map[string]interface{}{
		"name":      q.Name,
		"messages":  q.Messages,
		"consumers": q.Consumers,
	}, nil
}

// BindQueue creates a binding between a queue and an exchange.
// Messages routed to the exchange with a matching routing key
// will be delivered to the queue.
//
// JavaScript usage:
//
//	bindQueue({
//	    queue_name: "my-queue",
//	    exchange_name: "my-exchange",
//	    routing_key: "my.routing.key",
//	});
func (mi *ModuleInstance) BindQueue(options QueueBindOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.QueueBind(
		options.QueueName,
		options.RoutingKey,
		options.ExchangeName,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to bind queue %q to exchange %q: %w",
			options.QueueName, options.ExchangeName, err)
	}
	return nil
}

// UnbindQueue removes a binding between a queue and an exchange.
// Messages will no longer be routed from the exchange to this queue
// for the specified routing key.
//
// JavaScript usage:
//
//	unbindQueue({
//	    queue_name: "my-queue",
//	    exchange_name: "my-exchange",
//	    routing_key: "my.routing.key",
//	});
func (mi *ModuleInstance) UnbindQueue(options QueueUnbindOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.QueueUnbind(
		options.QueueName,
		options.RoutingKey,
		options.ExchangeName,
		options.Args,
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to unbind queue %q from exchange %q: %w",
			options.QueueName, options.ExchangeName, err)
	}
	return nil
}

// PurgeQueue removes all messages from a queue without deleting the
// queue itself. Returns the number of messages purged.
//
// JavaScript usage:
//
//	const count = purgeQueue("my-queue");
//	console.log(`Purged ${count} messages`);
func (mi *ModuleInstance) PurgeQueue(name string) (int, error) {
	ch, err := mi.getChannel()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = ch.Close()
	}()

	count, err := ch.QueuePurge(name, false /* noWait */)
	if err != nil {
		return 0, fmt.Errorf("amqp: failed to purge queue %q: %w", name, err)
	}
	return count, nil
}
