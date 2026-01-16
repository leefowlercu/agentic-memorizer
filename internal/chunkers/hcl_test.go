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

func TestHCLChunker_MalformedHCL(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_instance" "example" {
  ami = "ami-12345
  # Missing closing quote
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back and still produce chunks
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk from fallback, got %d", result.TotalChunks)
	}

	// Should have warnings about parse errors
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for malformed HCL")
	}
}

func TestHCLChunker_HeredocSyntax(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_instance" "example" {
  user_data = <<-EOF
    #!/bin/bash
    echo "Hello, World!"
    apt-get update
    apt-get install -y nginx
  EOF
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// Content should include the heredoc
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "Hello, World!") {
			t.Error("expected chunk to contain heredoc content")
		}
	}
}

func TestHCLChunker_CommentsOnlyFile(t *testing.T) {
	c := NewHCLChunker()
	content := `# This is a comment
# Another comment
// Single line comment
/* Block comment */
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce at least one chunk with the comments
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}
}

func TestHCLChunker_DeeplyNestedBlocks(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_iam_policy" "example" {
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject"
        ]
        Resource = "*"
      }
    ]
  })
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// All nested content should be in the chunk
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "jsonencode") {
			t.Error("expected chunk to contain jsonencode")
		}
		if !strings.Contains(chunk.Content, "s3:GetObject") {
			t.Error("expected chunk to contain nested action")
		}
	}
}

func TestHCLChunker_EscapedQuotes(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "description" {
  default = "This is a \"quoted\" value"
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}
}

func TestHCLChunker_ForExpression(t *testing.T) {
	c := NewHCLChunker()
	content := `locals {
  instance_ids = [for i in aws_instance.example : i.id]

  instance_map = {
    for i in aws_instance.example :
    i.tags.Name => i.id
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

	// For expressions should be preserved
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "for i in") {
			t.Error("expected chunk to contain for expression")
		}
	}
}

func TestHCLChunker_MultilineStrings(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "long_description" {
  default = <<EOT
This is a long description
that spans multiple lines
and contains various characters.
EOT
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}
}

func TestHCLChunker_OriginalSizeTracked(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "test" {
  type = string
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OriginalSize != len(content) {
		t.Errorf("expected OriginalSize %d, got %d", len(content), result.OriginalSize)
	}
}

func TestHCLChunker_TokenEstimatePopulated(t *testing.T) {
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

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.TokenEstimate <= 0 {
			t.Error("expected TokenEstimate to be positive")
		}
	}
}

func TestHCLChunker_ChunkIndexes(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "a" {
  type = string
}

variable "b" {
  type = number
}

variable "c" {
  type = bool
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify chunk indexes are sequential starting from 0
	for i, chunk := range result.Chunks {
		if chunk.Index != i {
			t.Errorf("expected chunk index %d, got %d", i, chunk.Index)
		}
	}
}

func TestHCLChunker_StartEndOffsets(t *testing.T) {
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
		if chunk.EndOffset <= chunk.StartOffset {
			t.Errorf("expected EndOffset > StartOffset, got StartOffset=%d EndOffset=%d",
				chunk.StartOffset, chunk.EndOffset)
		}
	}
}

func TestHCLChunker_LargeBlockSplitting(t *testing.T) {
	c := NewHCLChunker()

	// Create HCL with a very large resource block
	var largeResource strings.Builder
	largeResource.WriteString("resource \"aws_security_group\" \"large\" {\n")
	largeResource.WriteString("  name = \"large-sg\"\n")
	for i := range 30 {
		largeResource.WriteString("  ingress {\n")
		largeResource.WriteString("    from_port = " + string(rune('0'+i%10)) + "\n")
		largeResource.WriteString("    to_port = " + string(rune('0'+i%10)) + "\n")
		largeResource.WriteString("    protocol = \"tcp\"\n")
		largeResource.WriteString("    cidr_blocks = [\"10.0.0.0/8\"]\n")
		largeResource.WriteString("  }\n")
	}
	largeResource.WriteString("}\n")

	content := largeResource.String()

	// Use a small max chunk size to trigger splitting
	opts := ChunkOptions{MaxChunkSize: 500}

	result, err := c.Chunk(context.Background(), []byte(content), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be split into multiple chunks
	if result.TotalChunks < 2 {
		t.Errorf("expected content to be split into multiple chunks, got %d", result.TotalChunks)
	}

	// All chunks should have Infra metadata
	for i, chunk := range result.Chunks {
		if chunk.Metadata.Infra == nil {
			t.Errorf("chunk %d missing Infra metadata", i)
		}
	}
}

func TestHCLChunker_DynamicBlock(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_security_group" "example" {
  dynamic "ingress" {
    for_each = var.ingress_rules
    content {
      from_port   = ingress.value.from_port
      to_port     = ingress.value.to_port
      protocol    = ingress.value.protocol
      cidr_blocks = ingress.value.cidr_blocks
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

	// Dynamic block should be preserved
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "dynamic") {
			t.Error("expected chunk to contain dynamic block")
		}
	}
}

func TestHCLChunker_ResourceWithCount(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_instance" "example" {
  count = 3

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
		if chunk.Metadata.Infra.BlockType != "resource" {
			t.Errorf("expected block type 'resource', got %q", chunk.Metadata.Infra.BlockType)
		}
	}
}

func TestHCLChunker_ResourceWithForEach(t *testing.T) {
	c := NewHCLChunker()
	content := `resource "aws_iam_user" "example" {
  for_each = toset(["user1", "user2", "user3"])

  name = each.key
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// for_each should be preserved
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "for_each") {
			t.Error("expected chunk to contain for_each")
		}
	}
}

func TestHCLChunker_VariableWithValidation(t *testing.T) {
	c := NewHCLChunker()
	content := `variable "instance_type" {
  type        = string
  description = "EC2 instance type"
  default     = "t2.micro"

  validation {
    condition     = can(regex("^t[23]\\.", var.instance_type))
    error_message = "Instance type must be t2 or t3 series."
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

	// Validation block should be preserved
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "validation") {
			t.Error("expected chunk to contain validation block")
		}
	}
}

func TestHCLChunker_TFVarsFile(t *testing.T) {
	c := NewHCLChunker()
	// .tfvars files have simple key=value syntax
	content := `region = "us-east-1"
environment = "production"
instance_count = 3

tags = {
  Project = "example"
  Team    = "platform"
}
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce chunks even for simple key-value format
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}
}
