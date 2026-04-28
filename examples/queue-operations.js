// Queue Operations example
//
// Demonstrates all queue operations: declare, inspect, bind, unbind, purge, delete.
//
// Usage:
//   ./k6 run examples/queue-operations.js

import {
  connect, close,
  publish,
  declareQueue, deleteQueue, inspectQueue, purgeQueue,
  bindQueue, unbindQueue,
  declareExchange, deleteExchange,
} from 'k6/x/amqp';

export default function () {
  connect({ url: "amqp://guest:guest@localhost:5672/" });

  const queueName = 'k6-queue-demo';
  const exchangeName = 'k6-exchange-demo';

  // Declare queue with options
  const q = declareQueue({
    name: queueName,
    durable: false,          // Will not survive broker restart
    delete_when_unused: true, // Auto-delete when no consumers
  });
  console.log(`Declared queue: ${q.name}`);

  // Inspect queue
  const info = inspectQueue(queueName);
  console.log(`Queue info: messages=${info.messages}, consumers=${info.consumers}`);

  // Publish some messages
  for (let i = 0; i < 5; i++) {
    publish({
      queue_name: queueName,
      body: `Message ${i}`,
      content_type: "text/plain",
    });
  }
  console.log("Published 5 messages");

  // Purge all messages
  const purged = purgeQueue(queueName);
  console.log(`Purged ${purged} messages`);

  // Declare an exchange for binding demo
  declareExchange({ name: exchangeName, kind: "direct" });

  // Bind queue to exchange
  bindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: "demo.key",
  });
  console.log("Queue bound to exchange");

  // Unbind
  unbindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: "demo.key",
  });
  console.log("Queue unbound from exchange");

  // Clean up
  deleteExchange(exchangeName);
  deleteQueue(queueName);
  close();
  console.log("Done!");
}
