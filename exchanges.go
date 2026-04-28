// exchanges.go — Exchange operations for the xk6-amqp extension.
//
// Exchanges are the routing hubs in AMQP. Producers publish messages
// to exchanges, which then route them to queues based on bindings
// and routing keys.
//
// Exchange types supported by RabbitMQ:
//   - "direct":  routes to queues whose binding key exactly matches the routing key
//   - "fanout":  routes to all bound queues (ignores routing key)
//   - "topic":   routes based on wildcard pattern matching (* and #)
//   - "headers": routes based on message header attributes
//
// All methods are on ModuleInstance (per-VU, no shared state).
package amqp

import (
	"fmt"

	amqpDriver "github.com/rabbitmq/amqp091-go"
)

// ---------------------------------------------------------------------------
// Exchange Option Types
// ---------------------------------------------------------------------------

// DeclareExchangeOptions provides parameters when declaring (creating) an exchange.
type DeclareExchangeOptions struct {
	// Name is the exchange name.
	Name string `js:"name"`

	// Kind is the exchange type: "direct", "fanout", "topic", or "headers".
	Kind string `js:"kind"`

	// Durable exchanges survive broker restarts.
	Durable bool `js:"durable"`

	// AutoDelete: if true, the exchange is deleted when the last queue
	// bound to it is unbound.
	AutoDelete bool `js:"auto_delete"`

	// Internal exchanges cannot be published to directly by clients;
	// they can only receive messages from other exchanges via bindings.
	Internal bool `js:"internal"`

	// NoWait: if true, do not wait for server confirmation.
	NoWait bool `js:"no_wait"`

	// Args are optional arguments for the exchange.
	Args amqpDriver.Table `js:"args"`
}

// ExchangeBindOptions provides parameters when binding one exchange to another.
// This creates a routing link: messages published to the source exchange
// are also routed to the destination exchange.
type ExchangeBindOptions struct {
	// DestinationExchangeName receives routed messages.
	DestinationExchangeName string `js:"destination_exchange_name"`

	// SourceExchangeName is where messages originate.
	SourceExchangeName string `js:"source_exchange_name"`

	// RoutingKey is the routing pattern for the binding.
	RoutingKey string `js:"routing_key"`

	// NoWait: if true, do not wait for server confirmation.
	NoWait bool `js:"no_wait"`

	// Args are optional arguments for the binding.
	Args amqpDriver.Table `js:"args"`
}

// ExchangeUnbindOptions provides parameters when removing an exchange-to-exchange binding.
type ExchangeUnbindOptions struct {
	// DestinationExchangeName is the exchange that was receiving messages.
	DestinationExchangeName string `js:"destination_exchange_name"`

	// SourceExchangeName is the exchange that was sending messages.
	SourceExchangeName string `js:"source_exchange_name"`

	// RoutingKey must match the routing key used when the binding was created.
	RoutingKey string `js:"routing_key"`

	// NoWait: if true, do not wait for server confirmation.
	NoWait bool `js:"no_wait"`

	// Args must match the args used when the binding was created.
	Args amqpDriver.Table `js:"args"`
}

// ---------------------------------------------------------------------------
// Exchange Methods (on ModuleInstance)
// ---------------------------------------------------------------------------

// DeclareExchange creates or verifies an exchange on the AMQP server.
// If the exchange already exists with identical properties, this is a no-op.
//
// JavaScript usage:
//
//	declareExchange({
//	    name: "my-exchange",
//	    kind: "topic",
//	    durable: true,
//	});
func (mi *ModuleInstance) DeclareExchange(options DeclareExchangeOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.ExchangeDeclare(
		options.Name,
		options.Kind,
		options.Durable,
		options.AutoDelete,
		options.Internal,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to declare exchange %q (kind=%s): %w",
			options.Name, options.Kind, err)
	}
	return nil
}

// DeleteExchange removes an exchange from the AMQP server.
// Any existing bindings to the exchange are also removed.
//
// JavaScript usage:
//
//	deleteExchange("my-exchange");
func (mi *ModuleInstance) DeleteExchange(name string) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.ExchangeDelete(
		name,
		false, // ifUnused — delete even if queues are bound
		false, // noWait
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to delete exchange %q: %w", name, err)
	}
	return nil
}

// BindExchange creates a routing link from one exchange (source) to
// another (destination). Messages published to the source that match
// the routing key will also be delivered to the destination.
//
// JavaScript usage:
//
//	bindExchange({
//	    source_exchange_name: "source-exchange",
//	    destination_exchange_name: "dest-exchange",
//	    routing_key: "events.*",
//	});
func (mi *ModuleInstance) BindExchange(options ExchangeBindOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.ExchangeBind(
		options.DestinationExchangeName,
		options.RoutingKey,
		options.SourceExchangeName,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to bind exchange %q to %q: %w",
			options.SourceExchangeName, options.DestinationExchangeName, err)
	}
	return nil
}

// UnbindExchange removes a routing link between two exchanges.
//
// JavaScript usage:
//
//	unbindExchange({
//	    source_exchange_name: "source-exchange",
//	    destination_exchange_name: "dest-exchange",
//	    routing_key: "events.*",
//	});
func (mi *ModuleInstance) UnbindExchange(options ExchangeUnbindOptions) error {
	ch, err := mi.getChannel()
	if err != nil {
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.ExchangeUnbind(
		options.DestinationExchangeName,
		options.RoutingKey,
		options.SourceExchangeName,
		options.NoWait,
		options.Args,
	)
	if err != nil {
		return fmt.Errorf("amqp: failed to unbind exchange %q from %q: %w",
			options.SourceExchangeName, options.DestinationExchangeName, err)
	}
	return nil
}
