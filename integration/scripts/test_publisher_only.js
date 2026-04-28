// k6 Performance Test — Publisher Only
//
// This script simulates multiple sources (VUs) publishing messages
// continuously to RabbitMQ without consuming them.
// Useful for load testing the AMQP server or populating queues.
//
// Usage:
//   docker compose run --rm k6-performance run /scripts/test_publisher_only.js

import { connect, publish, declareQueue, close } from 'k6/x/amqp';
import { check, sleep } from 'k6';
import exec from 'k6/execution';
import { Counter, Trend } from 'k6/metrics';

// --- Custom Metrics ---
const messagesPublished = new Counter('amqp_messages_published');
const publishDuration = new Trend('amqp_publish_duration_ms');

// --- Configuration Profiles ---
// k6 scenarios allow precise control over VU concurrency and iterations.
// Uncomment ONE of the following profiles to use it:

// ---------------------------------------------------------
// Profile 1: Smoke Test
// Validates that the script and connection work.
// ---------------------------------------------------------
/*
export const options = {
  scenarios: {
    smoke_test: {
      executor: 'constant-vus',
      vus: 1,
      duration: '5s',
    },
  },
};
*/

// ---------------------------------------------------------
// Profile 2: Load Test
// Normal sustained load. Ramps up gradually to avoid initial connection storms.
// ---------------------------------------------------------

// export const options = {
//   scenarios: {
//     load_test: {
//       executor: 'ramping-vus',
//       startVUs: 0,
//       stages: [
//         { duration: '30s', target: 500 }, // Ramp up to 500 VUs over 30s
//         { duration: '2m', target: 500 },  // Hold 500 VUs for 2 minutes
//         { duration: '30s', target: 0 },   // Ramp down
//       ],
//     },
//   },
// };


// ---------------------------------------------------------
// Profile 3: Stress Test
// Pushes the system to its limits to find the breaking point.
// Uses up to 5000 VUs. Requires RabbitMQ ulimits tuning!
// ---------------------------------------------------------

// export const options = {
//   scenarios: {
//     stress_test: {
//       executor: 'ramping-vus',
//       startVUs: 0,
//       stages: [
//         { duration: '1m', target: 1000 }, // Ramp up to 1000 VUs
//         { duration: '2m', target: 3000 }, // Ramp up to 3000 VUs
//         { duration: '2m', target: 5000 }, // Push to 5000 VUs
//         { duration: '1m', target: 10000 }, // Push to 5000 VUs        
//         { duration: '1m', target: 0 },    // Ramp down quickly
//       ],
//     },
//   },
// };


// ---------------------------------------------------------
// Profile 4: Spike Test
// Simulates a sudden, massive surge in traffic.
// ---------------------------------------------------------
/*
export const options = {
  scenarios: {
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 100 },  // Baseline
        { duration: '10s', target: 4000 }, // SUDDEN SPIKE!
        { duration: '30s', target: 4000 }, // Hold the spike
        { duration: '10s', target: 100 },  // Drop back to baseline
        { duration: '10s', target: 0 },    // Cooldown
      ],
    },
  },
};
*/

// ---------------------------------------------------------
// Profile 5: Soak Test
// Sustained moderate load over a long period to find memory leaks.
// ---------------------------------------------------------
/*
export const options = {
  scenarios: {
    soak_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 400 }, // Ramp up
        { duration: '2h', target: 400 }, // Hold for 2 HOURS
        { duration: '2m', target: 0 },   // Ramp down
      ],
    },
  },
};
*/

// ---------------------------------------------------------
// Profile 6: Fixed Iterations Test (Targeted total messages)
// E.g., send exactly 100,000 messages total, shared among 100 VUs.
// ---------------------------------------------------------

export const options = {
  scenarios: {
    fixed_iterations: {
      executor: 'shared-iterations',
      vus: 500,
      iterations: 500000,
      maxDuration: '5m', // Give up if it takes longer than 5 minutes
    },
  },
};


// ---------------------------------------------------------

const amqpUrl = __ENV.AMQP_URL || "amqp://guest:guest@rabbitmq:5672/";

// In k6, variables in the global scope are initialized once per VU.
// This allows us to persist the AMQP connection across iterations for the same VU.
let isConnected = false;
const queueName = `k6-shared-inbox`;

export default function () {
  const vuId = exec.vu.idInInstance;
  const iteration = exec.vu.iterationInInstance;
  
  // Only connect and declare the queue on the first iteration of this VU.
  // Opening/closing TCP connections per iteration is extremely slow and will crash RabbitMQ.
  if (!isConnected) {
    connect({ url: amqpUrl });
    declareQueue({ name: queueName });
    isConnected = true;
  }

  // Payload: simulate a JSON event
  const messageBody = JSON.stringify({
    source: `device-${vuId}`,
    event_id: iteration,
    timestamp: Date.now(),
    data: "Simulated load testing payload"
  });

  const pubStart = Date.now();

  // Publish message
  try {
    publish({
      queue_name: queueName,
      body: messageBody,
      content_type: 'application/json',
      // persistent: true, // Uncomment to test persistent messages (slower)
    });

    const pubDuration = Date.now() - pubStart;
    publishDuration.add(pubDuration);
    messagesPublished.add(1);

    check(pubDuration, {
      'publish successful': (d) => d > -1, // Simple check that we got here
    });

  } catch (e) {
    console.error(`VU ${vuId} failed to publish: ${e}`);
  }

  // Sleep to simulate processing time between messages and prevent CPU pinning
  // Note: We DO NOT call close() here. The connection stays open for the next iteration.
  // RabbitMQ will close the connection when the k6 process exits.
  sleep(0.01); 
}
