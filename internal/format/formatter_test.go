package format

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFormatter is a test implementation of Formatter
type mockFormatter struct {
	name           string
	supportsColors bool
}

func (m *mockFormatter) Format(b Buildable) (string, error) {
	return fmt.Sprintf("formatted: %s", b.Type()), nil
}

func (m *mockFormatter) FormatMultiple(builders []Buildable) (string, error) {
	return fmt.Sprintf("formatted %d items", len(builders)), nil
}

func (m *mockFormatter) Name() string {
	return m.name
}

func (m *mockFormatter) SupportsColors() bool {
	return m.supportsColors
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	formatter := &mockFormatter{name: "test", supportsColors: false}
	registry.Register("test", formatter)

	retrieved, err := registry.Get("test")
	require.NoError(t, err)
	assert.Equal(t, formatter, retrieved)
}

func TestRegistry_GetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "formatter \"nonexistent\" not found")
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	registry.Register("formatter1", &mockFormatter{name: "formatter1"})
	registry.Register("formatter2", &mockFormatter{name: "formatter2"})
	registry.Register("formatter3", &mockFormatter{name: "formatter3"})

	names := registry.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "formatter1")
	assert.Contains(t, names, "formatter2")
	assert.Contains(t, names, "formatter3")
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()

	// Register formatters concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			name := fmt.Sprintf("formatter%d", i)
			registry.Register(name, &mockFormatter{name: name})
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	// Read formatters concurrently
	for i := 0; i < 10; i++ {
		go func(i int) {
			name := fmt.Sprintf("formatter%d", i)
			_, err := registry.Get(name)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Clear default registry for test isolation
	defaultRegistry = NewRegistry()

	formatter := &mockFormatter{name: "global-test"}
	RegisterFormatter("global-test", formatter)

	retrieved, err := GetFormatter("global-test")
	require.NoError(t, err)
	assert.Equal(t, formatter, retrieved)

	names := ListFormatters()
	assert.Contains(t, names, "global-test")
}
