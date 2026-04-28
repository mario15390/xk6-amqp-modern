// Basic example — Publish and Listen
//
// Demonstrates the simplest usage: connect, declare a queue,
// publish a message, and consume it.
//
// Usage:
//   ./k6 run examples/basic.js

import { connect, publish, listen, declareQueue, deleteQueue, close } from 'k6/x/amqp';

export default async function () {
  // Connect to RabbitMQ (each VU gets its own connection)
  const url = "amqp://guest:guest@localhost:5672/";
  connect({ url: url });
  console.log("Connected to " + url);

  // Declare a queue
  const queueName = 'k6-basic-test';
  const q = declareQueue({ name: queueName });
  console.log(`Queue ${q.name} ready (${q.messages} messages, ${q.consumers} consumers)`);

  // Publish a message
  publish({
    queue_name: queueName,
    body: "Ping from k6!",
    content_type: "text/plain",
  });
  console.log("Message published");

  // Listen for the message (returns a Promise)
  const received = await listen({
    queue_name: queueName,
    auto_ack: true,
  });
  console.log("Received: " + received);

  // Clean up
  deleteQueue(queueName);
  close();
}
