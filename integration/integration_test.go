//go:build integration
// +build integration

// Package integration contains tests that require a running RabbitMQ server.
//
// These tests validate the full AMQP lifecycle: connect, declare queues/exchanges,
// publish messages, consume messages, and clean up.
//
// Run with: go test -v -tags=integration -count=1 ./integration/...
//
// Environment variables:
//   - AMQP_URL: RabbitMQ connection string (default: amqp://guest:guest@localhost:5672/)
package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	amqpDriver "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getAMQPURL returns the AMQP connection URL from the environment
// or a sensible default for local Docker testing.
func getAMQPURL() string {
	url := os.Getenv("AMQP_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	return url
}

// waitForRabbitMQ blocks until RabbitMQ is reachable or the timeout expires.
// This is critical for Docker-based tests where RabbitMQ may still be starting.
func waitForRabbitMQ(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := amqpDriver.Dial(url)
		if err == nil {
			_ = conn.Close()
			t.Logf("✅ RabbitMQ is reachable at %s", url)
			return
		}
		t.Logf("⏳ Waiting for RabbitMQ at %s... (%v)", url, err)
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("❌ RabbitMQ not reachable at %s after %v", url, timeout)
}

// ---------------------------------------------------------------------------
// Connection Tests
// ---------------------------------------------------------------------------

func TestIntegration_Connect(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err, "Should connect to RabbitMQ")
	defer func() { _ = conn.Close() }()

	assert.NotNil(t, conn, "Connection should not be nil")
	t.Log("✅ Connection test passed")
}

func TestIntegration_ConnectAndClose(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)

	err = conn.Close()
	assert.NoError(t, err, "Should close connection without error")
	t.Log("✅ Connect and close test passed")
}

// ---------------------------------------------------------------------------
// Queue Tests
// ---------------------------------------------------------------------------

func TestIntegration_QueueDeclareInspectDelete(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	queueName := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

	// Declare
	q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	assert.Equal(t, queueName, q.Name)
	assert.Equal(t, 0, q.Messages)
	t.Logf("✅ Queue %q declared (messages=%d, consumers=%d)", q.Name, q.Messages, q.Consumers)

	// Inspect
	q2, err := ch.QueueInspect(queueName)
	require.NoError(t, err)
	assert.Equal(t, queueName, q2.Name)
	t.Logf("✅ Queue %q inspected (messages=%d, consumers=%d)", q2.Name, q2.Messages, q2.Consumers)

	// Delete
	_, err = ch.QueueDelete(queueName, false, false, false)
	assert.NoError(t, err)
	t.Logf("✅ Queue %q deleted", queueName)
}

func TestIntegration_QueuePurge(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	queueName := fmt.Sprintf("purge-test-%d", time.Now().UnixNano())

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _, _ = ch.QueueDelete(queueName, false, false, false) }()

	// Publish some messages
	for i := 0; i < 5; i++ {
		err = ch.PublishWithContext(context.Background(), "", queueName, false, false,
			amqpDriver.Publishing{Body: []byte(fmt.Sprintf("msg-%d", i))})
		require.NoError(t, err)
	}

	// Small delay for messages to be enqueued
	time.Sleep(200 * time.Millisecond)

	// Purge
	count, err := ch.QueuePurge(queueName, false)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "Should purge 5 messages")
	t.Logf("✅ Purged %d messages from queue %q", count, queueName)
}

// ---------------------------------------------------------------------------
// Exchange Tests
// ---------------------------------------------------------------------------

func TestIntegration_ExchangeDeclareAndDelete(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	exchangeName := fmt.Sprintf("exchange-test-%d", time.Now().UnixNano())

	// Declare a topic exchange
	err = ch.ExchangeDeclare(exchangeName, "topic", false, true, false, false, nil)
	require.NoError(t, err)
	t.Logf("✅ Exchange %q declared (type=topic)", exchangeName)

	// Delete
	err = ch.ExchangeDelete(exchangeName, false, false)
	assert.NoError(t, err)
	t.Logf("✅ Exchange %q deleted", exchangeName)
}

func TestIntegration_QueueBindToExchange(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	exchangeName := fmt.Sprintf("bind-ex-test-%d", time.Now().UnixNano())
	queueName := fmt.Sprintf("bind-q-test-%d", time.Now().UnixNano())
	routingKey := "test.key"

	// Setup
	err = ch.ExchangeDeclare(exchangeName, "direct", false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _ = ch.ExchangeDelete(exchangeName, false, false) }()

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _, _ = ch.QueueDelete(queueName, false, false, false) }()

	// Bind
	err = ch.QueueBind(queueName, routingKey, exchangeName, false, nil)
	require.NoError(t, err)
	t.Logf("✅ Queue %q bound to exchange %q with key %q", queueName, exchangeName, routingKey)

	// Unbind
	err = ch.QueueUnbind(queueName, routingKey, exchangeName, nil)
	assert.NoError(t, err)
	t.Logf("✅ Queue %q unbound from exchange %q", queueName, exchangeName)
}

// ---------------------------------------------------------------------------
// Publish & Consume Tests
// ---------------------------------------------------------------------------

func TestIntegration_PublishAndConsume(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	queueName := fmt.Sprintf("pubsub-test-%d", time.Now().UnixNano())
	testMessage := "Hello from integration test!"

	// Declare queue
	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _, _ = ch.QueueDelete(queueName, false, false, false) }()

	// Publish
	err = ch.PublishWithContext(context.Background(), "", queueName, false, false,
		amqpDriver.Publishing{
			ContentType: "text/plain",
			Body:        []byte(testMessage),
		})
	require.NoError(t, err)
	t.Logf("📤 Published message: %q", testMessage)

	// Consume
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	require.NoError(t, err)

	select {
	case msg := <-msgs:
		assert.Equal(t, testMessage, string(msg.Body))
		t.Logf("📥 Received message: %q", string(msg.Body))
		t.Log("✅ Publish and consume test passed")
	case <-time.After(10 * time.Second):
		t.Fatal("❌ Timed out waiting for message")
	}
}

func TestIntegration_PublishMultipleMessages(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	queueName := fmt.Sprintf("multi-test-%d", time.Now().UnixNano())
	messageCount := 10

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _, _ = ch.QueueDelete(queueName, false, false, false) }()

	// Publish multiple messages
	for i := 0; i < messageCount; i++ {
		body := fmt.Sprintf("message-%d", i)
		err = ch.PublishWithContext(context.Background(), "", queueName, false, false,
			amqpDriver.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			})
		require.NoError(t, err)
	}
	t.Logf("📤 Published %d messages", messageCount)

	// Consume all messages
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	require.NoError(t, err)

	received := 0
	timeout := time.After(10 * time.Second)

	for received < messageCount {
		select {
		case msg := <-msgs:
			expected := fmt.Sprintf("message-%d", received)
			assert.Equal(t, expected, string(msg.Body))
			received++
		case <-timeout:
			t.Fatalf("❌ Timed out: received %d/%d messages", received, messageCount)
		}
	}

	t.Logf("📥 Received %d/%d messages", received, messageCount)
	t.Log("✅ Multiple messages test passed")
}

func TestIntegration_PublishViaExchange(t *testing.T) {
	url := getAMQPURL()
	waitForRabbitMQ(t, url, 60*time.Second)

	conn, err := amqpDriver.Dial(url)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer func() { _ = ch.Close() }()

	exchangeName := fmt.Sprintf("via-ex-test-%d", time.Now().UnixNano())
	queueName := fmt.Sprintf("via-ex-q-test-%d", time.Now().UnixNano())
	routingKey := "test.route"
	testMessage := "Message via exchange"

	// Setup exchange and queue with binding
	err = ch.ExchangeDeclare(exchangeName, "direct", false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _ = ch.ExchangeDelete(exchangeName, false, false) }()

	_, err = ch.QueueDeclare(queueName, false, true, false, false, nil)
	require.NoError(t, err)
	defer func() { _, _ = ch.QueueDelete(queueName, false, false, false) }()

	err = ch.QueueBind(queueName, routingKey, exchangeName, false, nil)
	require.NoError(t, err)

	// Publish to exchange (not directly to queue)
	err = ch.PublishWithContext(context.Background(), exchangeName, routingKey, false, false,
		amqpDriver.Publishing{
			ContentType: "text/plain",
			Body:        []byte(testMessage),
		})
	require.NoError(t, err)
	t.Logf("📤 Published to exchange %q with key %q", exchangeName, routingKey)

	// Consume from queue
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	require.NoError(t, err)

	select {
	case msg := <-msgs:
		assert.Equal(t, testMessage, string(msg.Body))
		t.Logf("📥 Received via exchange: %q", string(msg.Body))
		t.Log("✅ Publish via exchange test passed")
	case <-time.After(10 * time.Second):
		t.Fatal("❌ Timed out waiting for message via exchange")
	}
}
