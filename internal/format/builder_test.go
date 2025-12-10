package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuilderType_String(t *testing.T) {
	tests := []struct {
		name        string
		builderType BuilderType
		want        string
	}{
		{"section", BuilderTypeSection, "section"},
		{"table", BuilderTypeTable, "table"},
		{"list", BuilderTypeList, "list"},
		{"progress", BuilderTypeProgress, "progress"},
		{"status", BuilderTypeStatus, "status"},
		{"error", BuilderTypeError, "error"},
		{"graph", BuilderTypeGraph, "graph"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.builderType.String())
		})
	}
}

// mockBuilder is a test implementation of Buildable
type mockBuilder struct {
	builderType BuilderType
	validateErr error
}

func (m *mockBuilder) Type() BuilderType {
	return m.builderType
}

func (m *mockBuilder) Validate() error {
	return m.validateErr
}

func TestBuildable_Interface(t *testing.T) {
	// Compile-time check that mockBuilder implements Buildable
	var _ Buildable = (*mockBuilder)(nil)

	mb := &mockBuilder{
		builderType: BuilderTypeSection,
		validateErr: nil,
	}

	assert.Equal(t, BuilderTypeSection, mb.Type())
	assert.NoError(t, mb.Validate())
}
