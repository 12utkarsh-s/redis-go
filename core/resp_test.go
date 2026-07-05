package core_test

import (
	"redis-go/core"
	"reflect"
	"testing"
)

func TestDecode_Success(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		expected     interface{}
		expectLength int // Internal check for DecodeOne if you want to test bytes consumed
	}{
		{
			name:     "Simple String",
			input:    []byte("+OK\r\n"),
			expected: "OK",
		},
		{
			name:     "Integer Positive",
			input:    []byte(":1000\r\n"),
			expected: int64(1000),
		},
		{
			name:     "Error Message",
			input:    []byte("-ERR unknown command\r\n"),
			expected: "ERR unknown command",
		},
		{
			name:     "Bulk String",
			input:    []byte("$5\r\nhello\r\n"),
			expected: "hello",
		},
		{
			name:     "Empty Bulk String",
			input:    []byte("$0\r\n\r\n"),
			expected: "",
		},
		{
			name:     "Array of Bulk Strings",
			input:    []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"),
			expected: []interface{}{"foo", "bar"},
		},
		{
			name:     "Mixed Array",
			input:    []byte("*3\r\n+1\r\n+PING\r\n$4\r\npong\r\n"),
			expected: []interface{}{"1", "PING", "pong"},
		},
		{
			name:     "Empty Array",
			input:    []byte("*0\r\n"),
			expected: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := core.Decode(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("Decode() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestDecodeOne_BytesConsumed(t *testing.T) {
	// This ensures that the position tracker (delta) moves perfectly
	// so the parser doesn't leave trailing data or over-consume.
	input := []byte("+OK\r\n:123\r\n")

	// First decode
	val1, delta1, err := core.DecodeOne(input)
	if err != nil {
		t.Fatalf("DecodeOne step 1 failed: %v", err)
	}
	if val1 != "OK" || delta1 != 5 {
		t.Errorf("Step 1 expected (\"OK\", 5), got (%v, %d)", val1, delta1)
	}

	// Second decode using the delta offset
	val2, delta2, err := core.DecodeOne(input[delta1:])
	if err != nil {
		t.Fatalf("DecodeOne step 2 failed: %v", err)
	}
	if val2 != int64(123) || delta2 != 6 {
		t.Errorf("Step 2 expected (123, 6), got (%v, %d)", val2, delta2)
	}
}

func TestDecode_Errors(t *testing.T) {
	t.Run("Empty Input Data", func(t *testing.T) {
		_, err := core.Decode([]byte{})
		if err == nil {
			t.Error("expected error for empty byte slice, got nil")
		}
	})
}

func TestDecodeStringArray_Success(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []string
	}{
		{
			name:     "Valid String Array",
			input:    []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"),
			expected: []string{"foo", "bar"},
		},
		{
			name:     "Empty String Array",
			input:    []byte("*0\r\n"),
			expected: []string{},
		},
		{
			name:     "Array with Simple Strings",
			input:    []byte("*2\r\n+OK\r\n+PING\r\n"),
			expected: []string{"OK", "PING"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := core.DecodeStringArray(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("DecodeStringArray() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestDecodeStringArray_PanicsAndErrors(t *testing.T) {
	t.Run("Fails on non-array input", func(t *testing.T) {
		// This will panic on value.([]interface{}) because it returns a string, not a slice
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected code to panic on non-array payload, but it didn't")
			}
		}()

		input := []byte("+JustAString\r\n")
		_, _ = core.DecodeStringArray(input)
	})

	t.Run("Fails on mixed type array", func(t *testing.T) {
		// This will panic on ts[i].(string) because index 0 is an int64
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected code to panic on integer inside array, but it didn't")
			}
		}()

		input := []byte("*2\r\n:100\r\n$3\r\nfoo\r\n")
		_, _ = core.DecodeStringArray(input)
	})

	t.Run("Returns error on empty payload", func(t *testing.T) {
		input := []byte{}
		_, err := core.DecodeStringArray(input)
		if err == nil {
			t.Error("expected underlying Decode error, got nil")
		}
	})
}
