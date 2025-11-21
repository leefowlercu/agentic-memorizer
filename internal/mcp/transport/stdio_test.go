package transport

import (
	"bufio"
	"bytes"
	"testing"
)

func TestStdioTransport_Write(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:  "write with newline",
			input: []byte(`{"jsonrpc":"2.0","id":1,"result":"test"}` + "\n"),
			want:  []byte(`{"jsonrpc":"2.0","id":1,"result":"test"}` + "\n"),
		},
		{
			name:  "write without newline",
			input: []byte(`{"jsonrpc":"2.0","id":1,"result":"test"}`),
			want:  []byte(`{"jsonrpc":"2.0","id":1,"result":"test"}` + "\n"),
		},
		{
			name:  "empty write",
			input: []byte{},
			want:  []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			transport := &StdioTransport{
				stdin:  nil, // Not used in Write tests
				stdout: buf,
			}

			err := transport.Write(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.Bytes(); !bytes.Equal(got, tt.want) {
				t.Errorf("Write() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStdioTransport_Read(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:  "read single line",
			input: `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n",
			want:  []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`),
		},
		{
			name:  "read with trailing newline",
			input: `{"test":"data"}` + "\n",
			want:  []byte(`{"test":"data"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewBufferString(tt.input))
			transport := &StdioTransport{
				stdin:  reader,
				stdout: nil, // Not used in Read tests
			}

			got, err := transport.Read()
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("Read() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStdioTransport_Close(t *testing.T) {
	transport := NewStdioTransport()
	if err := transport.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
