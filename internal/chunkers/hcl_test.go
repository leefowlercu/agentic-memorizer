package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHCLChunker_Name(t *testing.T) {
	c := NewHCLChunker()
	if c.Name() != "hcl" {
		t.Errorf("expected name 'hcl', got %q", c.Name())
	}
}

func TestHCLChunker_Priority(t *testing.T) {
	c := NewHCLChunker()
	if c.Priority() != 43 {
		t.Errorf("expected priority 43, got %d", c.Priority())
	}
}

func TestHCLChunker_CanHandle(t *testing.T) {
	c := NewHCLChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-hcl", "", true},
		{"application/x-hcl", "", true},
		{"text/x-terraform", "", true},
		{"application/x-terraform", "", true},
		{"", "main.tf", true},
		{"", "variables.tfvars", true},
		{"", "config.hcl", true},
		{"", "/path/to/main.tf", true},
		{"text/plain", "", false},
		{"", "go", false},
		{"", "python", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestHCLChunker_EmptyContent(t *testing.T) {
	c := NewHCLChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "hcl" {
		t.Errorf("expected chunker name 'hcl', got %q", result.ChunkerUsed)
	}
}

func TestHCLChunker_SingleResource(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_instance" "example" {
  ami           = "ami-12345678"
  instance_type = "t2.micro"
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra == nil {
			t.Fatal("expected Infra metadata to be set")
		}
		if chunk.Metadata.Infra.BlockType != "resource" {
			t.Errorf("expected block type 'resource', got %q", chunk.Metadata.Infra.BlockType)
		}
		if chunk.Metadata.Infra.ResourceType != "aws_instance" {
			t.Errorf("expected resource type 'aws_instance', got %q", chunk.Metadata.Infra.ResourceType)
		}
		if chunk.Metadata.Infra.ResourceName != "example" {
			t.Errorf("expected resource name 'example', got %q", chunk.Metadata.Infra.ResourceName)
		}
	}
}

func TestHCLChunker_MultipleBlocks(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "region" {
  type    = string
  default = "us-east-1"
}

resource "aws_s3_bucket" "main" {
  bucket = "my-bucket"
}

output "bucket_name" {
  value = aws_s3_bucket.main.bucket
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Check block types
	expectedTypes := []string{"variable", "resource", "output"}
	for i, expected := range expectedTypes {
		if i < result.TotalChunks {
			if result.Chunks[i].Metadata.Infra.BlockType != expected {
				t.Errorf("chunk %d: expected block type %q, got %q",
					i, expected, result.Chunks[i].Metadata.Infra.BlockType)
			}
		}
	}
}

func TestHCLChunker_DataSource(t *testing.T) {
	c := NewHCLChunker()
	content := `data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra.BlockType != "data" {
			t.Errorf("expected block type 'data', got %q", chunk.Metadata.Infra.BlockType)
		}
		if chunk.Metadata.Infra.ResourceType != "aws_ami" {
			t.Errorf("expected resource type 'aws_ami', got %q", chunk.Metadata.Infra.ResourceType)
		}
		if chunk.Metadata.Infra.ResourceName != "ubuntu" {
			t.Errorf("expected resource name 'ubuntu', got %q", chunk.Metadata.Infra.ResourceName)
		}
	}
}

func TestHCLChunker_Provider(t *testing.T) {
	c := NewHCLChunker()
	content := `provider "aws" {
  region = "us-east-1"
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra.BlockType != "provider" {
			t.Errorf("expected block type 'provider', got %q", chunk.Metadata.Infra.BlockType)
		}
		if chunk.Metadata.Infra.ResourceName != "aws" {
			t.Errorf("expected resource name 'aws', got %q", chunk.Metadata.Infra.ResourceName)
		}
	}
}

func TestHCLChunker_Module(t *testing.T) {
	c := NewHCLChunker()
	content := `module "vpc" {
  source = "./modules/vpc"
  cidr   = "10.0.0.0/16"
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra.BlockType != "module" {
			t.Errorf("expected block type 'module', got %q", chunk.Metadata.Infra.BlockType)
		}
		if chunk.Metadata.Infra.ResourceName != "vpc" {
			t.Errorf("expected resource name 'vpc', got %q", chunk.Metadata.Infra.ResourceName)
		}
	}
}

func TestHCLChunker_TestdataFixture(t *testing.T) {
	c := NewHCLChunker()

	// Read the testdata fixture
	fixturePath := filepath.Join("..", "..", "testdata", "devops", "sample.tf")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture should have: terraform, provider, 2 variables, 2 resources, 2 outputs = 8 blocks
	if result.TotalChunks < 6 {
		t.Errorf("expected at least 6 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify we have at least one resource block
	hasResource := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Infra != nil && chunk.Metadata.Infra.BlockType == "resource" {
			hasResource = true
			break
		}
	}
	if !hasResource {
		t.Error("expected at least one resource block")
	}
}

func TestHCLChunker_ContextCancellation(t *testing.T) {
	c := NewHCLChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `resource "aws_instance" "test" {
  ami = "ami-123"
}
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestHCLChunker_ChunkType(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "test" {
  type = string
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Type != ChunkTypeStructured {
			t.Errorf("expected ChunkTypeStructured, got %q", chunk.Metadata.Type)
		}
	}
}

func TestHCLChunker_NestedBlocks(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_security_group" "example" {
  name = "example"

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be one chunk since nested blocks are part of the resource
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk (nested blocks in resource), got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Verify the content includes the nested blocks
		if !strings.Contains(chunk.Content, "ingress") {
			t.Error("expected chunk to contain nested ingress block")
		}
		if !strings.Contains(chunk.Content, "egress") {
			t.Error("expected chunk to contain nested egress block")
		}
	}
}

func TestHCLChunker_TerraformBlock(t *testing.T) {
	c := NewHCLChunker()
	content := `terraform {
  required_version = ">= 1.0.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra.BlockType != "terraform" {
			t.Errorf("expected block type 'terraform', got %q", chunk.Metadata.Infra.BlockType)
		}
	}
}

func TestHCLChunker_LocalsBlock(t *testing.T) {
	c := NewHCLChunker()
	content := `locals {
  common_tags = {
    Environment = "prod"
    Project     = "myapp"
  }
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Infra.BlockType != "locals" {
			t.Errorf("expected block type 'locals', got %q", chunk.Metadata.Infra.BlockType)
		}
	}
}
