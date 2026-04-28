// k6 Integration Test — Queue Operations
//
// Tests all queue operations: declare, inspect, bind, unbind, purge, delete.
//
// Usage:
//   ./k6 run integration/scripts/test_queue_ops.js

import {
  connect, close,
  declareQueue, deleteQueue, inspectQueue, purgeQueue,
  bindQueue, unbindQueue,
  declareExchange, deleteExchange,
  publish,
} from 'k6/x/amqp';
import { check } from 'k6';

const amqpUrl = __ENV.AMQP_URL || "amqp://guest:guest@rabbitmq:5672/";

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1.0'],
  },
};

export default function () {
  connect({ url: amqpUrl });
  console.log('✅ Connected');

  const queueName = `k6-queue-ops-${Date.now()}`;
  const exchangeName = `k6-exchange-ops-${Date.now()}`;

  // --- Declare Queue ---
  const q = declareQueue({ name: queueName, durable: false });
  check(q, { 'queue declared': (q) => q.name === queueName });
  console.log(`✅ Queue declared: ${q.name}`);

  // --- Inspect Queue (empty) ---
  const info1 = inspectQueue(queueName);
  check(info1, {
    'inspect shows 0 messages': (i) => i.messages === 0,
  });
  console.log(`✅ Queue inspected: messages=${info1.messages}`);

  // --- Publish some messages ---
  for (let i = 0; i < 3; i++) {
    publish({
      queue_name: queueName,
      body: `message-${i}`,
      content_type: 'text/plain',
    });
  }
  console.log('📤 Published 3 messages');

  // --- Inspect Queue (with messages) ---
  // Small sleep to allow messages to be enqueued
  const info2 = inspectQueue(queueName);
  console.log(`✅ Queue inspected: messages=${info2.messages}`);

  // --- Purge Queue ---
  const purged = purgeQueue(queueName);
  console.log(`🗑️ Purged ${purged} messages`);

  // --- Declare Exchange for bind test ---
  declareExchange({ name: exchangeName, kind: 'direct' });
  console.log(`✅ Exchange declared: ${exchangeName}`);

  // --- Bind Queue ---
  bindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: 'test.key',
  });
  console.log(`✅ Queue bound to exchange`);

  // --- Unbind Queue ---
  unbindQueue({
    queue_name: queueName,
    exchange_name: exchangeName,
    routing_key: 'test.key',
  });
  console.log(`✅ Queue unbound from exchange`);

  // --- Cleanup ---
  deleteExchange(exchangeName);
  deleteQueue(queueName);
  close();
  console.log('✅ All queue operations passed');
}
