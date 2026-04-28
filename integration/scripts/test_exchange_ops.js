// k6 Integration Test — Exchange Operations
//
// Tests exchange lifecycle: declare, bind, unbind, delete.
//
// Usage:
//   ./k6 run integration/scripts/test_exchange_ops.js

import {
  connect, close,
  declareExchange, deleteExchange,
  bindExchange, unbindExchange,
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

  const srcExchange = `k6-src-exchange-${Date.now()}`;
  const dstExchange = `k6-dst-exchange-${Date.now()}`;

  // --- Declare Exchanges ---
  declareExchange({ name: srcExchange, kind: 'topic' });
  console.log(`✅ Source exchange declared: ${srcExchange}`);

  declareExchange({ name: dstExchange, kind: 'topic' });
  console.log(`✅ Destination exchange declared: ${dstExchange}`);

  // --- Bind Exchanges ---
  bindExchange({
    source_exchange_name: srcExchange,
    destination_exchange_name: dstExchange,
    routing_key: 'events.#',
  });
  console.log('✅ Exchanges bound');

  // --- Unbind Exchanges ---
  unbindExchange({
    source_exchange_name: srcExchange,
    destination_exchange_name: dstExchange,
    routing_key: 'events.#',
  });
  console.log('✅ Exchanges unbound');

  // --- Delete Exchanges ---
  deleteExchange(dstExchange);
  deleteExchange(srcExchange);
  console.log('✅ Exchanges deleted');

  close();
  console.log('✅ All exchange operations passed');
}
