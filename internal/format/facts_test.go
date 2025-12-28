package format

import (
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactsContent(t *testing.T) {
	index := &types.FactsIndex{
		Generated: time.Now(),
		Facts:     []types.Fact{},
		Stats: types.FactStats{
			TotalFacts: 0,
			MaxFacts:   50,
		},
	}

	fc := NewFactsContent(index)

	require.NotNil(t, fc)
	assert.Equal(t, index, fc.Index)
	assert.Equal(t, BuilderTypeFacts, fc.Type())
}

func TestNewFactsContent_Nil(t *testing.T) {
	fc := NewFactsContent(nil)

	require.NotNil(t, fc)
	assert.Nil(t, fc.Index)
}

func TestFactsContent_Type(t *testing.T) {
	fc := &FactsContent{}

	assert.Equal(t, BuilderTypeFacts, fc.Type())
	assert.Equal(t, "facts", fc.Type().String())
}

func TestFactsContent_Validate_Valid(t *testing.T) {
	tests := []struct {
		name  string
		index *types.FactsIndex
	}{
		{
			name: "empty facts",
			index: &types.FactsIndex{
				Generated: time.Now(),
				Facts:     []types.Fact{},
				Stats: types.FactStats{
					TotalFacts: 0,
					MaxFacts:   50,
				},
			},
		},
		{
			name: "with facts",
			index: &types.FactsIndex{
				Generated: time.Now(),
				Facts: []types.Fact{
					{
						ID:        "fact-1",
						Content:   "This is a test fact",
						CreatedAt: time.Now(),
						Source:    "cli",
					},
					{
						ID:        "fact-2",
						Content:   "Another test fact",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						Source:    "cli",
					},
				},
				Stats: types.FactStats{
					TotalFacts: 2,
					MaxFacts:   50,
				},
			},
		},
		{
			name: "nil facts slice",
			index: &types.FactsIndex{
				Generated: time.Now(),
				Facts:     nil,
				Stats: types.FactStats{
					TotalFacts: 0,
					MaxFacts:   50,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := NewFactsContent(tt.index)
			err := fc.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestFactsContent_Validate_NilIndex(t *testing.T) {
	fc := NewFactsContent(nil)

	err := fc.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FactsIndex cannot be nil")
}

func TestFactsContent_Validate_DirectNilIndex(t *testing.T) {
	fc := &FactsContent{Index: nil}

	err := fc.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "FactsIndex cannot be nil")
}

func TestFactsContent_ImplementsBuildable(t *testing.T) {
	// Verify FactsContent implements the Buildable interface
	var _ Buildable = (*FactsContent)(nil)

	fc := NewFactsContent(&types.FactsIndex{
		Generated: time.Now(),
		Facts:     []types.Fact{},
		Stats:     types.FactStats{TotalFacts: 0, MaxFacts: 50},
	})

	// Test interface methods
	assert.Equal(t, BuilderTypeFacts, fc.Type())
	assert.NoError(t, fc.Validate())
}
