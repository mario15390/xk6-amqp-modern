// k6 Performance & Functional Test — Multi-VU Publish/Listen
//
// This script is designed for YOU to run manually for functional and
// performance testing. It simulates multiple VUs each with their own
// AMQP connection, publishing and consuming messages concurrently.
//
// Each VU:
//   1. Connects to RabbitMQ (per-VU isolated connection)
//   2. Declares its own queue (named by VU ID)
//   3. Publishes a message
//   4. Listens for the message (async Promise)
//   5. Validates the received message
//
// Usage:
//   # Quick smoke test (1 VU, 1 iteration)
//   ./k6 run integration/scripts/test_multi_vu.js
//
//   # Performance test (10 VUs, 30 seconds)
//   ./k6 run integration/scripts/test_multi_vu.js --vus 10 --duration 30s
//
//   # Stress test (50 VUs, 1 minute)
//   ./k6 run integration/scripts/test_multi_vu.js --vus 50 --duration 1m
//
// Environment:
//   AMQP_URL — RabbitMQ URL (default: amqp://guest:guest@rabbitmq:5672/)

import { connect, publish, listen, declareQueue, deleteQueue, close } from 'k6/x/amqp';
import { check, sleep } from 'k6';
import exec from 'k6/execution';
import { Counter, Trend } from 'k6/metrics';

// --- Custom Metrics ---
const messagesPublished = new Counter('amqp_messages_published');
const messagesReceived = new Counter('amqp_messages_received');
const publishDuration = new Trend('amqp_publish_duration_ms');
const listenDuration = new Trend('amqp_listen_duration_ms');

// --- Configuration ---
const amqpUrl = __ENV.AMQP_URL || "amqp://guest:guest@rabbitmq:5672/";

export const options = {
  // Default: gentle test. Override with --vus and --duration flags.
  vus: 5,
  iterations: 20,
  thresholds: {
    checks: ['rate>=0.95'],                     // 95% of checks must pass
    'amqp_publish_duration_ms': ['p(95)<500'],  // 95th percentile publish < 500ms
    'amqp_listen_duration_ms': ['p(95)<2000'],  // 95th percentile listen < 2s
  },
};

// --- Setup: connect per VU ---
// Note: In k6, each VU runs the default function independently.
// The new per-VU module instance means each VU gets its own AMQP connection.
let isConnected = false;
let queueName = "";

export default async function () {
  const vuId = exec.vu.idInInstance;
  const iteration = exec.vu.iterationInInstance;
  
  if (!isConnected) {
    // Connect (each VU has its own connection thanks to ModuleInstance)
    connect({ url: amqpUrl });

    // Declare a unique queue for this VU (reused across iterations)
    queueName = `k6-perf-vu${vuId}-${Date.now()}`;
    const q = declareQueue({ name: queueName });
    check(q, {
      'queue declared': (q) => q.name === queueName,
    });
    isConnected = true;
  }

  // Publish
  const messageBody = `VU=${vuId} iter=${iteration} ts=${Date.now()}`;
  const pubStart = Date.now();

  publish({
    queue_name: queueName,
    body: messageBody,
    content_type: 'text/plain',
  });

  const pubDuration = Date.now() - pubStart;
  publishDuration.add(pubDuration);
  messagesPublished.add(1);

  // Listen (async)
  const lisStart = Date.now();
  const received = await listen({
    queue_name: queueName,
    auto_ack: true,
  });
  const lisDuration = Date.now() - lisStart;
  listenDuration.add(lisDuration);
  messagesReceived.add(1);

  // Validate
  const passed = check(received, {
    'received message matches': (msg) => msg === messageBody,
  });

  if (passed) {
    console.log(`✅ VU${vuId} iter${iteration}: pub=${pubDuration}ms, listen=${lisDuration}ms`);
  } else {
    console.log(`❌ VU${vuId} iter${iteration}: expected="${messageBody}" got="${received}"`);
  }

  // Note: We DO NOT call close() or deleteQueue() here. 
  // Opening and closing connections/queues per iteration is an anti-pattern for load testing
  // and will severely degrade throughput. RabbitMQ handles the teardown on process exit.
}
