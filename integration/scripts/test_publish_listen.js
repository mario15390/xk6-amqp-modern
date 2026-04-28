// k6 Integration Test — Publish and Listen
//
// This script tests the full AMQP lifecycle using the modernized extension:
//   1. Connect to RabbitMQ
//   2. Declare a queue
//   3. Publish a message
//   4. Consume the message (await Promise)
//   5. Verify the message content
//   6. Clean up and close
//
// Usage:
//   ./k6 run integration/scripts/test_publish_listen.js
//
// Environment:
//   AMQP_URL — RabbitMQ connection string (default: amqp://guest:guest@rabbitmq:5672/)

import { connect, publish, listen, declareQueue, deleteQueue, close } from 'k6/x/amqp';
import { check } from 'k6';

// Use environment variable or default to Docker service name
const amqpUrl = __ENV.AMQP_URL || "amqp://guest:guest@rabbitmq:5672/";
const queueName = `k6-integration-test-${Date.now()}`;

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1.0'], // All checks must pass
  },
};

export default async function () {
  // Step 1: Connect
  connect({ url: amqpUrl });
  console.log(`✅ Connected to ${amqpUrl}`);

  // Step 2: Declare queue
  const q = declareQueue({ name: queueName });
  console.log(`✅ Queue declared: ${q.name} (messages=${q.messages})`);

  check(q, {
    'queue was declared': (q) => q.name === queueName,
    'queue starts empty': (q) => q.messages === 0,
  });

  // Step 3: Publish message
  const testBody = 'Hello from k6 integration test!';
  publish({
    queue_name: queueName,
    body: testBody,
    content_type: 'text/plain',
  });
  console.log(`📤 Published: "${testBody}"`);

  // Step 4: Consume message (async — uses event loop)
  const received = await listen({
    queue_name: queueName,
    auto_ack: true,
  });
  console.log(`📥 Received: "${received}"`);

  check(received, {
    'message content matches': (msg) => msg === testBody,
  });

  // Step 5: Clean up
  deleteQueue(queueName);
  console.log(`🗑️ Queue ${queueName} deleted`);

  close();
  console.log('✅ Connection closed');
}
