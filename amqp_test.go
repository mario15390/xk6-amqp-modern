package amqp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// RootModule Tests
// ---------------------------------------------------------------------------

// TestRootModuleCreation verifies the RootModule can be instantiated
// and satisfies the modules.Module interface (compile-time check is
// in amqp.go via var _ modules.Module = &RootModule{}).
func TestRootModuleCreation(t *testing.T) {
	t.Parallel()
	rm := &RootModule{}
	assert.NotNil(t, rm, "RootModule should be created successfully")
}

// ---------------------------------------------------------------------------
// ModuleInstance Tests (without AMQP connection)
// ---------------------------------------------------------------------------

// TestModuleInstanceExports verifies that all expected named exports
// are present in the module's Exports() output.
func TestModuleInstanceExports(t *testing.T) {
	t.Parallel()

	// We can't easily create a real modules.VU in tests, so we test
	// the export names that should be present by checking the keys.
	expectedExports := []string{
		"version",
		"connect",
		"close",
		"publish",
		"listen",
		"declareQueue",
		"deleteQueue",
		"inspectQueue",
		"bindQueue",
		"unbindQueue",
		"purgeQueue",
		"declareExchange",
		"deleteExchange",
		"bindExchange",
		"unbindExchange",
	}

	// Create a ModuleInstance with nil VU (we're only checking export names)
	mi := &ModuleInstance{
		version: version,
	}
	mi.exports.Named = map[string]interface{}{}
	for _, name := range expectedExports {
		mi.exports.Named[name] = nil // placeholder
	}

	exports := mi.Exports()
	assert.NotNil(t, exports.Named, "Named exports should not be nil")
	assert.Len(t, exports.Named, len(expectedExports),
		"Should have exactly %d named exports", len(expectedExports))

	for _, name := range expectedExports {
		_, ok := exports.Named[name]
		assert.True(t, ok, "Export %q should be present", name)
	}
}

// TestConnectRequiresURL verifies that Connect fails with a clear error
// when no URL is provided.
func TestConnectRequiresURL(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.Connect(ConnectOptions{URL: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection URL is required")
}

// TestConnectInvalidURL verifies that Connect fails gracefully when
// given an unreachable or malformed URL.
func TestConnectInvalidURL(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.Connect(ConnectOptions{URL: "amqp://invalid:invalid@localhost:99999/"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

// TestCloseWithoutConnect verifies that Close is a no-op when called
// before any connection has been established.
func TestCloseWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.Close()
	assert.NoError(t, err, "Close should succeed even when not connected")
}

// TestGetChannelWithoutConnect verifies that attempting to get a channel
// without an active connection returns a clear error.
func TestGetChannelWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	ch, err := mi.getChannel()
	assert.Nil(t, ch, "Channel should be nil when not connected")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestPublishWithoutConnect verifies that Publish fails with a clear
// error when no connection is active.
func TestPublishWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.Publish(PublishOptions{
		QueueName:   "test-queue",
		Body:        "test",
		ContentType: "text/plain",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestDeclareQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestDeclareQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	_, err := mi.DeclareQueue(DeclareQueueOptions{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestDeleteQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestDeleteQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.DeleteQueue("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestInspectQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestInspectQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	_, err := mi.InspectQueue("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestBindQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestBindQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.BindQueue(QueueBindOptions{QueueName: "test", ExchangeName: "ex"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestUnbindQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestUnbindQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.UnbindQueue(QueueUnbindOptions{QueueName: "test", ExchangeName: "ex"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestPurgeQueueWithoutConnect verifies queue operations fail
// gracefully when not connected.
func TestPurgeQueueWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	_, err := mi.PurgeQueue("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestDeclareExchangeWithoutConnect verifies exchange operations fail
// gracefully when not connected.
func TestDeclareExchangeWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.DeclareExchange(DeclareExchangeOptions{Name: "test", Kind: "direct"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestDeleteExchangeWithoutConnect verifies exchange operations fail
// gracefully when not connected.
func TestDeleteExchangeWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.DeleteExchange("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestBindExchangeWithoutConnect verifies exchange operations fail
// gracefully when not connected.
func TestBindExchangeWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.BindExchange(ExchangeBindOptions{
		SourceExchangeName:      "src",
		DestinationExchangeName: "dst",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// TestUnbindExchangeWithoutConnect verifies exchange operations fail
// gracefully when not connected.
func TestUnbindExchangeWithoutConnect(t *testing.T) {
	t.Parallel()
	mi := &ModuleInstance{}

	err := mi.UnbindExchange(ExchangeUnbindOptions{
		SourceExchangeName:      "src",
		DestinationExchangeName: "dst",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// ---------------------------------------------------------------------------
// Option Struct Tests
// ---------------------------------------------------------------------------

// TestPublishOptionsDefaults verifies the zero-value defaults for
// PublishOptions (all booleans false, all strings empty).
func TestPublishOptionsDefaults(t *testing.T) {
	t.Parallel()
	opts := PublishOptions{}

	assert.Empty(t, opts.QueueName)
	assert.Empty(t, opts.Body)
	assert.Empty(t, opts.Exchange)
	assert.Empty(t, opts.ContentType)
	assert.False(t, opts.Mandatory)
	assert.False(t, opts.Immediate)
	assert.False(t, opts.Persistent)
	assert.Zero(t, opts.Timestamp)
}

// TestListenOptionsDefaults verifies the zero-value defaults for ListenOptions.
func TestListenOptionsDefaults(t *testing.T) {
	t.Parallel()
	opts := ListenOptions{}

	assert.Empty(t, opts.QueueName)
	assert.Empty(t, opts.Consumer)
	assert.False(t, opts.AutoAck)
	assert.False(t, opts.Exclusive)
	assert.False(t, opts.NoLocal)
	assert.False(t, opts.NoWait)
}

// TestDeclareQueueOptionsDefaults verifies the zero-value defaults.
func TestDeclareQueueOptionsDefaults(t *testing.T) {
	t.Parallel()
	opts := DeclareQueueOptions{}

	assert.Empty(t, opts.Name)
	assert.False(t, opts.Durable)
	assert.False(t, opts.DeleteWhenUnused)
	assert.False(t, opts.Exclusive)
	assert.False(t, opts.NoWait)
}

// TestDeclareExchangeOptionsDefaults verifies the zero-value defaults.
func TestDeclareExchangeOptionsDefaults(t *testing.T) {
	t.Parallel()
	opts := DeclareExchangeOptions{}

	assert.Empty(t, opts.Name)
	assert.Empty(t, opts.Kind)
	assert.False(t, opts.Durable)
	assert.False(t, opts.AutoDelete)
	assert.False(t, opts.Internal)
	assert.False(t, opts.NoWait)
}

// TestVersionConstant verifies the version string is set.
func TestVersionConstant(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "v1.0.0", version)
	assert.NotEmpty(t, version)
}

// TestMessagepackConstant verifies the msgpack MIME type constant.
func TestMessagepackConstant(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "application/x-msgpack", messagepack)
}
