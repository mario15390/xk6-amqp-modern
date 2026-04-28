// Exchange Routing example
//
// Demonstrates topic exchange routing: messages published to an exchange
// are routed to queues based on routing key patterns.
//
// Usage:
//   ./k6 run examples/exchange-routing.js

import {
  connect, close,
  publish, listen,
  declareQueue, deleteQueue,
  declareExchange, deleteExchange,
  bindQueue, unbindQueue,
} from 'k6/x/amqp';

export default async function () {
  connect({ url: "amqp://guest:guest@localhost:5672/" });

  const exchangeName = 'k6-topic-exchange';
  const queueName = 'k6-events-queue';
  const routingPattern = 'events.#'; // Matches events.foo, events.foo.bar, etc.
  const publishKey = 'events.user.login';

  // Setup: exchange + queue + binding
  declareExchange({ name: exchangeName, kind: "topic" });
  declareQueue({ name: queueName });
  bindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: routingPattern,
  });
  console.log(`Setup: ${queueName} bound to ${exchangeName} with pattern "${routingPattern}"`);

  // Publish to exchange with a specific routing key
  publish({
    exchange: exchangeName,
    queue_name: publishKey, // This is used as the routing key
    body: JSON.stringify({ event: "user.login", user: "k6-tester" }),
    content_type: "application/json",
  });
  console.log(`Published to ${exchangeName} with key "${publishKey}"`);

  // Consume from the bound queue
  const msg = await listen({
    queue_name: queueName,
    auto_ack: true,
  });
  console.log("Received: " + msg);

  // Cleanup
  unbindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: routingPattern,
  });
  deleteQueue(queueName);
  deleteExchange(exchangeName);
  close();
  console.log("Done!");
}
