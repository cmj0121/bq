package bq

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantOrder ByteOrder
		wantCodes []rune
		wantErr   bool
	}{
		{
			name:      "little endian with all codes",
			format:    "<bBhHiIqQ",
			wantOrder: LittleEndian,
			wantCodes: []rune{'b', 'B', 'h', 'H', 'i', 'I', 'q', 'Q'},
			wantErr:   false,
		},
		{
			name:      "big endian single code",
			format:    ">i",
			wantOrder: BigEndian,
			wantCodes: []rune{'i'},
			wantErr:   false,
		},
		{
			name:      "native order",
			format:    "@hH",
			wantOrder: NativeOrder,
			wantCodes: []rune{'h', 'H'},
			wantErr:   false,
		},
		{
			name:      "no prefix defaults to native",
			format:    "bB",
			wantOrder: NativeOrder,
			wantCodes: []rune{'b', 'B'},
			wantErr:   false,
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: true,
		},
		{
			name:    "only prefix",
			format:  "<",
			wantErr: true,
		},
		{
			name:    "unknown code",
			format:  "<x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if expr.Order != tt.wantOrder {
				t.Errorf("Parse() Order = %v, want %v", expr.Order, tt.wantOrder)
			}

			if len(expr.Formats) != len(tt.wantCodes) {
				t.Errorf("Parse() Formats len = %v, want %v", len(expr.Formats), len(tt.wantCodes))
				return
			}

			for i, fc := range expr.Formats {
				if fc.Code != tt.wantCodes[i] {
					t.Errorf("Parse() Formats[%d].Code = %c, want %c", i, fc.Code, tt.wantCodes[i])
				}
			}
		})
	}
}

func TestExpr_Read(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		data    []byte
		want    []any
		wantErr bool
	}{
		{
			name:   "signed char",
			format: "<b",
			data:   []byte{0xFF},
			want:   []any{int8(-1)},
		},
		{
			name:   "unsigned char",
			format: "<B",
			data:   []byte{0xFF},
			want:   []any{uint8(255)},
		},
		{
			name:   "little endian short",
			format: "<h",
			data:   []byte{0x01, 0x02},
			want:   []any{int16(0x0201)},
		},
		{
			name:   "big endian short",
			format: ">h",
			data:   []byte{0x01, 0x02},
			want:   []any{int16(0x0102)},
		},
		{
			name:   "little endian unsigned short",
			format: "<H",
			data:   []byte{0xFF, 0xFF},
			want:   []any{uint16(0xFFFF)},
		},
		{
			name:   "little endian int",
			format: "<i",
			data:   []byte{0x01, 0x02, 0x03, 0x04},
			want:   []any{int32(0x04030201)},
		},
		{
			name:   "big endian int",
			format: ">i",
			data:   []byte{0x01, 0x02, 0x03, 0x04},
			want:   []any{int32(0x01020304)},
		},
		{
			name:   "little endian unsigned int",
			format: "<I",
			data:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
			want:   []any{uint32(0xFFFFFFFF)},
		},
		{
			name:   "little endian long",
			format: "<q",
			data:   []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want:   []any{int64(0x0807060504030201)},
		},
		{
			name:   "big endian long",
			format: ">q",
			data:   []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want:   []any{int64(0x0102030405060708)},
		},
		{
			name:   "little endian unsigned long",
			format: "<Q",
			data:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			want:   []any{uint64(0xFFFFFFFFFFFFFFFF)},
		},
		{
			name:   "multiple formats",
			format: "<bBh",
			data:   []byte{0xFF, 0xFF, 0x01, 0x02},
			want:   []any{int8(-1), uint8(255), int16(0x0201)},
		},
		{
			name:    "insufficient data",
			format:  "<i",
			data:    []byte{0x01, 0x02},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := expr.Read(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("Read() len = %v, want %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Read() [%d] = %v (%T), want %v (%T)", i, got[i], got[i], tt.want[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatCodeSize(t *testing.T) {
	tests := []struct {
		code rune
		size int
	}{
		{'b', 1},
		{'B', 1},
		{'h', 2},
		{'H', 2},
		{'i', 4},
		{'I', 4},
		{'q', 8},
		{'Q', 8},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			expr, err := Parse(string(tt.code))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if expr.Formats[0].Size != tt.size {
				t.Errorf("Size = %v, want %v", expr.Formats[0].Size, tt.size)
			}
		})
	}
}

func TestPrettyPrint(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		data     []byte
		contains []string
	}{
		{
			name:   "single signed char",
			format: "<b",
			data:   []byte{0xFF},
			contains: []string{
				"Index", "Code", "Type", "Value", "Hex",
				"0", "b", "int8", "-1", "0xff",
			},
		},
		{
			name:   "multiple formats",
			format: "<bBh",
			data:   []byte{0x01, 0x02, 0x03, 0x04},
			contains: []string{
				"0", "b", "int8", "1", "0x01",
				"1", "B", "uint8", "2", "0x02",
				"2", "h", "int16", "1027", "0x0403",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			values, err := expr.Read(bytes.NewReader(tt.data))
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}

			var buf bytes.Buffer
			if err := PrettyPrint(&buf, expr, values); err != nil {
				t.Fatalf("PrettyPrint() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("PrettyPrint() output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestFormatHex(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"int8 positive", int8(1), "0x01"},
		{"int8 negative", int8(-1), "0xff"},
		{"uint8", uint8(255), "0xff"},
		{"int16", int16(0x0102), "0x0102"},
		{"int16 negative", int16(-1), "0xffff"},
		{"uint16", uint16(0xFFFF), "0xffff"},
		{"int32", int32(0x01020304), "0x01020304"},
		{"int32 negative", int32(-1), "0xffffffff"},
		{"uint32", uint32(0xFFFFFFFF), "0xffffffff"},
		{"int64", int64(0x0102030405060708), "0x0102030405060708"},
		{"int64 negative", int64(-1), "0xffffffffffffffff"},
		{"uint64", uint64(0xFFFFFFFFFFFFFFFF), "0xffffffffffffffff"},
		{"string", "hello", "[68 65 6c 6c 6f]"},
		{"string empty", "", "[]"},
		{"unknown type", struct{}{}, "N/A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHex(tt.val)
			if got != tt.want {
				t.Errorf("formatHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenizer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []Token
	}{
		{
			name:  "simple format",
			input: "bH",
			tokens: []Token{
				{Type: TokenFormat, Value: "b"},
				{Type: TokenFormat, Value: "H"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "format with order",
			input: "<bH",
			tokens: []Token{
				{Type: TokenOrder, Value: "<"},
				{Type: TokenFormat, Value: "b"},
				{Type: TokenFormat, Value: "H"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "pipe and object",
			input: "bH | {0 -> key, 1 -> value}",
			tokens: []Token{
				{Type: TokenFormat, Value: "b"},
				{Type: TokenFormat, Value: "H"},
				{Type: TokenPipe, Value: "|"},
				{Type: TokenLBrace, Value: "{"},
				{Type: TokenNumber, Value: "0"},
				{Type: TokenArrow, Value: "->"},
				{Type: TokenIdent, Value: "key"},
				{Type: TokenComma, Value: ","},
				{Type: TokenNumber, Value: "1"},
				{Type: TokenArrow, Value: "->"},
				{Type: TokenIdent, Value: "value"},
				{Type: TokenRBrace, Value: "}"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "all byte orders",
			input: "< > @",
			tokens: []Token{
				{Type: TokenOrder, Value: "<"},
				{Type: TokenOrder, Value: ">"},
				{Type: TokenOrder, Value: "@"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "function call with parentheses",
			input: "parse(<bH)",
			tokens: []Token{
				{Type: TokenIdent, Value: "parse"},
				{Type: TokenLParen, Value: "("},
				{Type: TokenOrder, Value: "<"},
				{Type: TokenFormat, Value: "b"},
				{Type: TokenFormat, Value: "H"},
				{Type: TokenRParen, Value: ")"},
				{Type: TokenEOF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewTokenizer(tt.input)
			for i, want := range tt.tokens {
				got, err := tokenizer.Next()
				if err != nil {
					t.Fatalf("Next() error = %v at token %d", err, i)
				}
				if got.Type != want.Type {
					t.Errorf("Token[%d].Type = %v, want %v", i, got.Type, want.Type)
				}
				if want.Value != "" && got.Value != want.Value {
					t.Errorf("Token[%d].Value = %q, want %q", i, got.Value, want.Value)
				}
			}
		})
	}
}

func TestTokenizerPeek(t *testing.T) {
	tokenizer := NewTokenizer("bH")

	// Peek should not consume the token
	tok1, err := tokenizer.Peek()
	if err != nil {
		t.Fatalf("Peek() error = %v", err)
	}
	if tok1.Type != TokenFormat || tok1.Value != "b" {
		t.Errorf("Peek() = %v, want TokenFormat 'b'", tok1)
	}

	// Next should return the same token
	tok2, err := tokenizer.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if tok2.Type != tok1.Type || tok2.Value != tok1.Value {
		t.Errorf("Next() after Peek() = %v, want %v", tok2, tok1)
	}
}

func TestParseExpression(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple format",
			input:   "bH",
			wantErr: false,
		},
		{
			name:    "format with byte order",
			input:   "<bH",
			wantErr: false,
		},
		{
			name:    "pipe with object",
			input:   "bH | {0 -> key, 1 -> value}",
			wantErr: false,
		},
		{
			name:    "multiple fields",
			input:   "<bHi | {0 -> first, 1 -> second, 2 -> third}",
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing format codes",
			input:   "| {0 -> x}",
			wantErr: true,
		},
		{
			name:    "invalid object syntax",
			input:   "bH | {x -> key}",
			wantErr: true,
		},
		{
			name:    "missing arrow",
			input:   "bH | {0 key}",
			wantErr: true,
		},
		{
			name:    "missing field name",
			input:   "bH | {0 ->}",
			wantErr: true,
		},
		// parse() function tests
		{
			name:    "parse function simple",
			input:   "parse(bH)",
			wantErr: false,
		},
		{
			name:    "parse function with byte order",
			input:   "parse(<bH)",
			wantErr: false,
		},
		{
			name:    "parse function with array",
			input:   "parse(4B)",
			wantErr: false,
		},
		{
			name:    "parse function with pipe",
			input:   "parse(<bH) | {0 -> key, 1 -> value}",
			wantErr: false,
		},
		{
			name:    "unknown function",
			input:   "unknown(bH)",
			wantErr: true,
		},
		{
			name:    "parse function missing closing paren",
			input:   "parse(bH",
			wantErr: true,
		},
		{
			name:    "parse function empty args",
			input:   "parse()",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && node == nil {
				t.Errorf("ParseExpression() returned nil node without error")
			}
		})
	}
}

func TestParseExpressionAST(t *testing.T) {
	// Test that parsing produces the correct AST structure
	t.Run("format only", func(t *testing.T) {
		node, err := ParseExpression("<bH")
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		formatNode, ok := node.(*FormatNode)
		if !ok {
			t.Fatalf("Expected *FormatNode, got %T", node)
		}

		if formatNode.Order != LittleEndian {
			t.Errorf("Order = %v, want LittleEndian", formatNode.Order)
		}

		if len(formatNode.Formats) != 2 {
			t.Errorf("Formats len = %d, want 2", len(formatNode.Formats))
		}
	})

	t.Run("pipe with object", func(t *testing.T) {
		node, err := ParseExpression("bH | {0 -> key, 1 -> value}")
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		pipeNode, ok := node.(*PipeNode)
		if !ok {
			t.Fatalf("Expected *PipeNode, got %T", node)
		}

		// Check left side is FormatNode
		_, ok = pipeNode.Left.(*FormatNode)
		if !ok {
			t.Errorf("Left node: expected *FormatNode, got %T", pipeNode.Left)
		}

		// Check right side is ObjectNode
		objectNode, ok := pipeNode.Right.(*ObjectNode)
		if !ok {
			t.Fatalf("Right node: expected *ObjectNode, got %T", pipeNode.Right)
		}

		if len(objectNode.Fields) != 2 {
			t.Errorf("Fields len = %d, want 2", len(objectNode.Fields))
		}

		if objectNode.Fields[0].Index != 0 || objectNode.Fields[0].Name != "key" {
			t.Errorf("Fields[0] = %v, want {0, key}", objectNode.Fields[0])
		}

		if objectNode.Fields[1].Index != 1 || objectNode.Fields[1].Name != "value" {
			t.Errorf("Fields[1] = %v, want {1, value}", objectNode.Fields[1])
		}
	})
}

func TestObjectNodeEval(t *testing.T) {
	tests := []struct {
		name       string
		fields     []FieldDef
		values     []any
		wantFields []ObjectField
		wantErr    bool
	}{
		{
			name: "simple object",
			fields: []FieldDef{
				{Index: 0, Name: "key"},
				{Index: 1, Name: "value"},
			},
			values: []any{int8(-1), uint16(513)},
			wantFields: []ObjectField{
				{Name: "key", Value: int8(-1)},
				{Name: "value", Value: uint16(513)},
			},
			wantErr: false,
		},
		{
			name: "reorder fields",
			fields: []FieldDef{
				{Index: 1, Name: "second"},
				{Index: 0, Name: "first"},
			},
			values: []any{int8(1), int8(2)},
			wantFields: []ObjectField{
				{Name: "second", Value: int8(2)},
				{Name: "first", Value: int8(1)},
			},
			wantErr: false,
		},
		{
			name: "index out of range",
			fields: []FieldDef{
				{Index: 5, Name: "invalid"},
			},
			values:  []any{int8(1)},
			wantErr: true,
		},
		{
			name: "negative index",
			fields: []FieldDef{
				{Index: -1, Name: "invalid"},
			},
			values:  []any{int8(1)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ObjectNode{Fields: tt.fields}
			result, err := node.Eval(nil, tt.values)

			if (err != nil) != tt.wantErr {
				t.Errorf("Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			obj, ok := result.(*Object)
			if !ok {
				t.Fatalf("Eval() returned %T, want *Object", result)
			}

			if len(obj.Fields) != len(tt.wantFields) {
				t.Errorf("Fields len = %d, want %d", len(obj.Fields), len(tt.wantFields))
				return
			}

			for i, want := range tt.wantFields {
				got := obj.Fields[i]
				if got.Name != want.Name || got.Value != want.Value {
					t.Errorf("Fields[%d] = %v, want %v", i, got, want)
				}
			}
		})
	}
}

func TestPipeNodeEval(t *testing.T) {
	// Test full pipeline: FormatNode | ObjectNode
	data := []byte{0xFF, 0x01, 0x02}

	node, err := ParseExpression("<bH | {0 -> key, 1 -> value}")
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}

	result, err := node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	obj, ok := result.(*Object)
	if !ok {
		t.Fatalf("Eval() returned %T, want *Object", result)
	}

	if len(obj.Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2", len(obj.Fields))
	}

	// Check key field (int8 -1)
	if obj.Fields[0].Name != "key" {
		t.Errorf("Fields[0].Name = %q, want %q", obj.Fields[0].Name, "key")
	}
	if v, ok := obj.Fields[0].Value.(int8); !ok || v != -1 {
		t.Errorf("Fields[0].Value = %v, want int8(-1)", obj.Fields[0].Value)
	}

	// Check value field (uint16 513)
	if obj.Fields[1].Name != "value" {
		t.Errorf("Fields[1].Name = %q, want %q", obj.Fields[1].Name, "value")
	}
	if v, ok := obj.Fields[1].Value.(uint16); !ok || v != 513 {
		t.Errorf("Fields[1].Value = %v, want uint16(513)", obj.Fields[1].Value)
	}
}

func TestPrettyPrintResult(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		data     []byte
		contains []string
	}{
		{
			name:  "format only",
			input: "<bH",
			data:  []byte{0xFF, 0x01, 0x02},
			contains: []string{
				"Name", "Code", "Type", "Value", "Hex",
				"0", "b", "int8", "-1", "0xff",
				"1", "H", "uint16", "513", "0x0201",
			},
		},
		{
			name:  "pipe with object",
			input: "<bH | {0 -> key, 1 -> value}",
			data:  []byte{0xFF, 0x01, 0x02},
			contains: []string{
				"Name", "Code", "Type", "Value", "Hex",
				"key", "b", "int8", "-1", "0xff",
				"value", "H", "uint16", "513", "0x0201",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if err != nil {
				t.Fatalf("ParseExpression() error = %v", err)
			}

			result, err := node.Eval(bytes.NewReader(tt.data), nil)
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}

			var buf bytes.Buffer
			if err := PrettyPrintResult(&buf, node, result); err != nil {
				t.Fatalf("PrettyPrintResult() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("PrettyPrintResult() output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestInferTypeInfo(t *testing.T) {
	tests := []struct {
		val      any
		wantCode rune
		wantType string
	}{
		{int8(0), 'b', "int8"},
		{uint8(0), 'B', "uint8"},
		{int16(0), 'h', "int16"},
		{uint16(0), 'H', "uint16"},
		{int32(0), 'i', "int32"},
		{uint32(0), 'I', "uint32"},
		{int64(0), 'q', "int64"},
		{uint64(0), 'Q', "uint64"},
		{"hello", 's', "string"},
		{struct{}{}, '?', "unknown"},
		// Array types
		{[]int8{}, 'b', "[]int8"},
		{[]uint8{}, 'B', "[]uint8"},
		{[]int16{}, 'h', "[]int16"},
		{[]uint16{}, 'H', "[]uint16"},
		{[]int32{}, 'i', "[]int32"},
		{[]uint32{}, 'I', "[]uint32"},
		{[]int64{}, 'q', "[]int64"},
		{[]uint64{}, 'Q', "[]uint64"},
	}

	for _, tt := range tests {
		t.Run(tt.wantType, func(t *testing.T) {
			code, typeName := inferTypeInfo(tt.val)
			if code != tt.wantCode {
				t.Errorf("inferTypeInfo() code = %c, want %c", code, tt.wantCode)
			}
			if typeName != tt.wantType {
				t.Errorf("inferTypeInfo() type = %s, want %s", typeName, tt.wantType)
			}
		})
	}
}

func TestParseWithArrayCount(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantCount []int
		wantCodes []rune
		wantErr   bool
	}{
		{
			name:      "single array",
			format:    "4B",
			wantCount: []int{4},
			wantCodes: []rune{'B'},
		},
		{
			name:      "mixed single and array",
			format:    "<b4Bh",
			wantCount: []int{1, 4, 1},
			wantCodes: []rune{'b', 'B', 'h'},
		},
		{
			name:      "multiple arrays",
			format:    "2h3i",
			wantCount: []int{2, 3},
			wantCodes: []rune{'h', 'i'},
		},
		{
			name:      "large count",
			format:    "100B",
			wantCount: []int{100},
			wantCodes: []rune{'B'},
		},
		{
			name:    "zero count",
			format:  "0B",
			wantErr: true,
		},
		{
			name:    "count without format",
			format:  "4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(expr.Formats) != len(tt.wantCodes) {
				t.Errorf("Parse() formats len = %d, want %d", len(expr.Formats), len(tt.wantCodes))
				return
			}

			for i, fc := range expr.Formats {
				if fc.Code != tt.wantCodes[i] {
					t.Errorf("Parse() Formats[%d].Code = %c, want %c", i, fc.Code, tt.wantCodes[i])
				}
				if fc.Count != tt.wantCount[i] {
					t.Errorf("Parse() Formats[%d].Count = %d, want %d", i, fc.Count, tt.wantCount[i])
				}
			}
		})
	}
}

func TestExpr_ReadArray(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		data    []byte
		want    []any
		wantErr bool
	}{
		{
			name:   "4 unsigned chars",
			format: "<4B",
			data:   []byte{0x01, 0x02, 0x03, 0x04},
			want:   []any{[]uint8{1, 2, 3, 4}},
		},
		{
			name:   "2 signed shorts little endian",
			format: "<2h",
			data:   []byte{0x01, 0x00, 0xFF, 0xFF},
			want:   []any{[]int16{1, -1}},
		},
		{
			name:   "2 unsigned shorts big endian",
			format: ">2H",
			data:   []byte{0x00, 0x01, 0x00, 0x02},
			want:   []any{[]uint16{1, 2}},
		},
		{
			name:   "mixed single and array",
			format: "<b4B",
			data:   []byte{0xFF, 0x01, 0x02, 0x03, 0x04},
			want:   []any{int8(-1), []uint8{1, 2, 3, 4}},
		},
		{
			name:   "2 signed ints",
			format: "<2i",
			data:   []byte{0x01, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF},
			want:   []any{[]int32{1, -1}},
		},
		{
			name:   "2 signed longs",
			format: "<2q",
			data:   []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			want:   []any{[]int64{1, -1}},
		},
		{
			name:    "insufficient data for array",
			format:  "<4i",
			data:    []byte{0x01, 0x02, 0x03, 0x04},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := expr.Read(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("Read() len = %v, want %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if !compareValues(got[i], tt.want[i]) {
					t.Errorf("Read() [%d] = %v (%T), want %v (%T)", i, got[i], got[i], tt.want[i], tt.want[i])
				}
			}
		})
	}
}

// compareValues compares two values, handling slices specially.
func compareValues(a, b any) bool {
	switch av := a.(type) {
	case []int8:
		bv, ok := b.([]int8)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []uint8:
		bv, ok := b.([]uint8)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []int16:
		bv, ok := b.([]int16)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []uint16:
		bv, ok := b.([]uint16)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []int32:
		bv, ok := b.([]int32)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []uint32:
		bv, ok := b.([]uint32)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []int64:
		bv, ok := b.([]int64)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []uint64:
		bv, ok := b.([]uint64)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

func TestFormatHexArray(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"[]uint8", []uint8{0x01, 0x02, 0x03}, "[01 02 03]"},
		{"[]int8", []int8{-1, 0, 1}, "[ff 00 01]"},
		{"[]uint16", []uint16{0x0102, 0x0304}, "[0102 0304]"},
		{"[]int16", []int16{-1, 1}, "[ffff 0001]"},
		{"[]uint32", []uint32{0x01020304}, "[01020304]"},
		{"[]int32", []int32{-1}, "[ffffffff]"},
		{"[]uint64", []uint64{0x0102030405060708}, "[0102030405060708]"},
		{"[]int64", []int64{-1}, "[ffffffffffffffff]"},
		{"empty []uint8", []uint8{}, "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHex(tt.val)
			if got != tt.want {
				t.Errorf("formatHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseExpressionWithArray(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple array",
			input:   "4B",
			wantErr: false,
		},
		{
			name:    "mixed with pipe",
			input:   "b4B | {0 -> header, 1 -> data}",
			wantErr: false,
		},
		{
			name:    "array with byte order",
			input:   "<2H",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && node == nil {
				t.Errorf("ParseExpression() returned nil node without error")
			}
		})
	}
}

func TestParseFunctionEval(t *testing.T) {
	// Test that parse() function evaluates correctly
	data := []byte{0xFF, 0x01, 0x02}

	tests := []struct {
		name  string
		input string
		want  []any
	}{
		{
			name:  "parse function simple",
			input: "parse(<bH)",
			want:  []any{int8(-1), uint16(513)},
		},
		{
			name:  "parse function with array",
			input: "parse(<b2B)",
			want:  []any{int8(-1), []uint8{1, 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if err != nil {
				t.Fatalf("ParseExpression() error = %v", err)
			}

			result, err := node.Eval(bytes.NewReader(data), nil)
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}

			values, ok := result.([]any)
			if !ok {
				t.Fatalf("Eval() returned %T, want []any", result)
			}

			if len(values) != len(tt.want) {
				t.Fatalf("Eval() len = %d, want %d", len(values), len(tt.want))
			}

			for i := range values {
				if !compareValues(values[i], tt.want[i]) {
					t.Errorf("Eval()[%d] = %v (%T), want %v (%T)", i, values[i], values[i], tt.want[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseFunctionWithPipe(t *testing.T) {
	// Test parse() with pipe to object
	data := []byte{0xFF, 0x01, 0x02}

	node, err := ParseExpression("parse(<bH) | {0 -> key, 1 -> value}")
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}

	result, err := node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	obj, ok := result.(*Object)
	if !ok {
		t.Fatalf("Eval() returned %T, want *Object", result)
	}

	if len(obj.Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2", len(obj.Fields))
	}

	if obj.Fields[0].Name != "key" {
		t.Errorf("Fields[0].Name = %q, want %q", obj.Fields[0].Name, "key")
	}
	if v, ok := obj.Fields[0].Value.(int8); !ok || v != -1 {
		t.Errorf("Fields[0].Value = %v, want int8(-1)", obj.Fields[0].Value)
	}

	if obj.Fields[1].Name != "value" {
		t.Errorf("Fields[1].Name = %q, want %q", obj.Fields[1].Name, "value")
	}
	if v, ok := obj.Fields[1].Value.(uint16); !ok || v != 513 {
		t.Errorf("Fields[1].Value = %v, want uint16(513)", obj.Fields[1].Value)
	}
}

// Tests for nested objects feature

func TestTokenizerColon(t *testing.T) {
	// Test that colon token is recognized
	tokenizer := NewTokenizer("nested: {0 -> x}")

	tokens := []Token{
		{Type: TokenIdent, Value: "nested"},
		{Type: TokenColon, Value: ":"},
		{Type: TokenLBrace, Value: "{"},
		{Type: TokenNumber, Value: "0"},
		{Type: TokenArrow, Value: "->"},
		{Type: TokenIdent, Value: "x"},
		{Type: TokenRBrace, Value: "}"},
		{Type: TokenEOF},
	}

	for i, want := range tokens {
		got, err := tokenizer.Next()
		if err != nil {
			t.Fatalf("Next() error = %v at token %d", err, i)
		}
		if got.Type != want.Type {
			t.Errorf("Token[%d].Type = %v, want %v", i, got.Type, want.Type)
		}
		if want.Value != "" && got.Value != want.Value {
			t.Errorf("Token[%d].Value = %q, want %q", i, got.Value, want.Value)
		}
	}
}

func TestTokenizerNestedObject(t *testing.T) {
	// Test tokenizing a full nested object expression
	input := "<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}"
	tokenizer := NewTokenizer(input)

	expectedTokens := []Token{
		{Type: TokenOrder, Value: "<"},
		{Type: TokenFormat, Value: "b"},
		{Type: TokenFormat, Value: "H"},
		{Type: TokenFormat, Value: "B"},
		{Type: TokenPipe, Value: "|"},
		{Type: TokenLBrace, Value: "{"},
		{Type: TokenNumber, Value: "0"},
		{Type: TokenArrow, Value: "->"},
		{Type: TokenIdent, Value: "header"},
		{Type: TokenComma, Value: ","},
		{Type: TokenIdent, Value: "nested"},
		{Type: TokenColon, Value: ":"},
		{Type: TokenLBrace, Value: "{"},
		{Type: TokenNumber, Value: "1"},
		{Type: TokenArrow, Value: "->"},
		{Type: TokenIdent, Value: "length"},
		{Type: TokenComma, Value: ","},
		{Type: TokenNumber, Value: "2"},
		{Type: TokenArrow, Value: "->"},
		{Type: TokenIdent, Value: "flag"},
		{Type: TokenRBrace, Value: "}"},
		{Type: TokenRBrace, Value: "}"},
		{Type: TokenEOF},
	}

	for i, want := range expectedTokens {
		got, err := tokenizer.Next()
		if err != nil {
			t.Fatalf("Next() error = %v at token %d", err, i)
		}
		if got.Type != want.Type {
			t.Errorf("Token[%d].Type = %v, want %v", i, got.Type, want.Type)
		}
		if want.Value != "" && got.Value != want.Value {
			t.Errorf("Token[%d].Value = %q, want %q", i, got.Value, want.Value)
		}
	}
}

func TestParseExpressionNestedObject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple nested object",
			input:   "bH | {nested: {0 -> x, 1 -> y}}",
			wantErr: false,
		},
		{
			name:    "mixed flat and nested",
			input:   "<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}",
			wantErr: false,
		},
		{
			name:    "deeply nested",
			input:   "bHiI | {0 -> a, level1: {1 -> b, level2: {2 -> c, 3 -> d}}}",
			wantErr: false,
		},
		{
			name:    "multiple nested at same level",
			input:   "<bHiI | {first: {0 -> a, 1 -> b}, second: {2 -> c, 3 -> d}}",
			wantErr: false,
		},
		{
			name:    "nested missing colon",
			input:   "bH | {nested {0 -> x}}",
			wantErr: true,
		},
		{
			name:    "nested missing opening brace",
			input:   "bH | {nested: 0 -> x}",
			wantErr: true,
		},
		{
			name:    "nested missing closing brace",
			input:   "bH | {nested: {0 -> x}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && node == nil {
				t.Errorf("ParseExpression() returned nil node without error")
			}
		})
	}
}

func TestParseExpressionNestedAST(t *testing.T) {
	// Test that nested object parsing produces correct AST structure
	t.Run("simple nested", func(t *testing.T) {
		node, err := ParseExpression("bH | {nested: {0 -> x, 1 -> y}}")
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		pipeNode, ok := node.(*PipeNode)
		if !ok {
			t.Fatalf("Expected *PipeNode, got %T", node)
		}

		objNode, ok := pipeNode.Right.(*ObjectNode)
		if !ok {
			t.Fatalf("Expected *ObjectNode on right, got %T", pipeNode.Right)
		}

		if len(objNode.Fields) != 1 {
			t.Errorf("Expected 1 field, got %d", len(objNode.Fields))
		}

		// Check nested field
		field := objNode.Fields[0]
		if field.Name != "nested" {
			t.Errorf("Field name = %q, want %q", field.Name, "nested")
		}
		if field.Nested == nil {
			t.Fatal("Expected nested object, got nil")
		}
		if len(field.Nested.Fields) != 2 {
			t.Errorf("Nested fields len = %d, want 2", len(field.Nested.Fields))
		}
	})

	t.Run("mixed flat and nested", func(t *testing.T) {
		node, err := ParseExpression("<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}")
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		pipeNode, ok := node.(*PipeNode)
		if !ok {
			t.Fatalf("Expected *PipeNode, got %T", node)
		}

		objNode, ok := pipeNode.Right.(*ObjectNode)
		if !ok {
			t.Fatalf("Expected *ObjectNode on right, got %T", pipeNode.Right)
		}

		if len(objNode.Fields) != 2 {
			t.Fatalf("Expected 2 fields, got %d", len(objNode.Fields))
		}

		// Check first field (flat)
		if objNode.Fields[0].Name != "header" || objNode.Fields[0].Index != 0 || objNode.Fields[0].Nested != nil {
			t.Errorf("Fields[0] = %+v, want flat field named 'header' at index 0", objNode.Fields[0])
		}

		// Check second field (nested)
		if objNode.Fields[1].Name != "nested" || objNode.Fields[1].Nested == nil {
			t.Errorf("Fields[1] = %+v, want nested field named 'nested'", objNode.Fields[1])
		}
		if len(objNode.Fields[1].Nested.Fields) != 2 {
			t.Errorf("Nested fields len = %d, want 2", len(objNode.Fields[1].Nested.Fields))
		}
	})
}

func TestNestedObjectNodeEval(t *testing.T) {
	tests := []struct {
		name      string
		fields    []FieldDef
		values    []any
		wantErr   bool
		checkFunc func(t *testing.T, obj *Object)
	}{
		{
			name: "simple nested",
			fields: []FieldDef{
				{Name: "nested", Nested: &ObjectNode{
					Fields: []FieldDef{
						{Index: 0, Name: "x"},
						{Index: 1, Name: "y"},
					},
				}},
			},
			values:  []any{int8(1), int8(2)},
			wantErr: false,
			checkFunc: func(t *testing.T, obj *Object) {
				if len(obj.Fields) != 1 {
					t.Fatalf("Fields len = %d, want 1", len(obj.Fields))
				}
				nested, ok := obj.Fields[0].Value.(*Object)
				if !ok {
					t.Fatalf("Expected nested *Object, got %T", obj.Fields[0].Value)
				}
				if len(nested.Fields) != 2 {
					t.Errorf("Nested fields len = %d, want 2", len(nested.Fields))
				}
				if nested.Fields[0].Value != int8(1) {
					t.Errorf("Nested x = %v, want 1", nested.Fields[0].Value)
				}
				if nested.Fields[1].Value != int8(2) {
					t.Errorf("Nested y = %v, want 2", nested.Fields[1].Value)
				}
			},
		},
		{
			name: "mixed flat and nested",
			fields: []FieldDef{
				{Index: 0, Name: "header"},
				{Name: "nested", Nested: &ObjectNode{
					Fields: []FieldDef{
						{Index: 1, Name: "length"},
						{Index: 2, Name: "flag"},
					},
				}},
			},
			values:  []any{int8(-1), uint16(513), uint8(3)},
			wantErr: false,
			checkFunc: func(t *testing.T, obj *Object) {
				if len(obj.Fields) != 2 {
					t.Fatalf("Fields len = %d, want 2", len(obj.Fields))
				}
				// Check flat field
				if obj.Fields[0].Name != "header" || obj.Fields[0].Value != int8(-1) {
					t.Errorf("Fields[0] = %+v, want header=-1", obj.Fields[0])
				}
				// Check nested field
				nested, ok := obj.Fields[1].Value.(*Object)
				if !ok {
					t.Fatalf("Expected nested *Object, got %T", obj.Fields[1].Value)
				}
				if nested.Fields[0].Value != uint16(513) {
					t.Errorf("Nested length = %v, want 513", nested.Fields[0].Value)
				}
				if nested.Fields[1].Value != uint8(3) {
					t.Errorf("Nested flag = %v, want 3", nested.Fields[1].Value)
				}
			},
		},
		{
			name: "nested index out of range",
			fields: []FieldDef{
				{Name: "nested", Nested: &ObjectNode{
					Fields: []FieldDef{
						{Index: 99, Name: "invalid"},
					},
				}},
			},
			values:  []any{int8(1)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ObjectNode{Fields: tt.fields}
			result, err := node.Eval(nil, tt.values)

			if (err != nil) != tt.wantErr {
				t.Errorf("Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			obj, ok := result.(*Object)
			if !ok {
				t.Fatalf("Eval() returned %T, want *Object", result)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, obj)
			}
		})
	}
}

func TestNestedPipeNodeEval(t *testing.T) {
	// Test full pipeline with nested objects
	data := []byte{0xFF, 0x01, 0x02, 0x03}

	node, err := ParseExpression("<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}")
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}

	result, err := node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	obj, ok := result.(*Object)
	if !ok {
		t.Fatalf("Eval() returned %T, want *Object", result)
	}

	if len(obj.Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2", len(obj.Fields))
	}

	// Check header field (int8 -1)
	if obj.Fields[0].Name != "header" {
		t.Errorf("Fields[0].Name = %q, want %q", obj.Fields[0].Name, "header")
	}
	if v, ok := obj.Fields[0].Value.(int8); !ok || v != -1 {
		t.Errorf("Fields[0].Value = %v, want int8(-1)", obj.Fields[0].Value)
	}

	// Check nested object
	if obj.Fields[1].Name != "nested" {
		t.Errorf("Fields[1].Name = %q, want %q", obj.Fields[1].Name, "nested")
	}
	nested, ok := obj.Fields[1].Value.(*Object)
	if !ok {
		t.Fatalf("Fields[1].Value expected *Object, got %T", obj.Fields[1].Value)
	}

	// Check nested fields
	if len(nested.Fields) != 2 {
		t.Fatalf("Nested fields len = %d, want 2", len(nested.Fields))
	}
	if nested.Fields[0].Name != "length" {
		t.Errorf("Nested[0].Name = %q, want %q", nested.Fields[0].Name, "length")
	}
	if v, ok := nested.Fields[0].Value.(uint16); !ok || v != 513 {
		t.Errorf("Nested[0].Value = %v, want uint16(513)", nested.Fields[0].Value)
	}
	if nested.Fields[1].Name != "flag" {
		t.Errorf("Nested[1].Name = %q, want %q", nested.Fields[1].Name, "flag")
	}
	if v, ok := nested.Fields[1].Value.(uint8); !ok || v != 3 {
		t.Errorf("Nested[1].Value = %v, want uint8(3)", nested.Fields[1].Value)
	}
}

func TestPrettyPrintNestedObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		data     []byte
		contains []string
	}{
		{
			name:  "simple nested object",
			input: "bH | {nested: {0 -> x, 1 -> y}}",
			data:  []byte{0x01, 0x02, 0x00},
			contains: []string{
				"Name", "Code", "Type", "Value", "Hex",
				"nested", "object",
				"x", "b", "int8", "1", "0x01",
				"y", "H", "uint16", "2", "0x0002",
			},
		},
		{
			name:  "mixed flat and nested",
			input: "<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}",
			data:  []byte{0xFF, 0x01, 0x02, 0x03},
			contains: []string{
				"header", "b", "int8", "-1", "0xff",
				"nested", "object",
				"length", "H", "uint16", "513", "0x0201",
				"flag", "B", "uint8", "3", "0x03",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if err != nil {
				t.Fatalf("ParseExpression() error = %v", err)
			}

			result, err := node.Eval(bytes.NewReader(tt.data), nil)
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}

			var buf bytes.Buffer
			if err := PrettyPrintResult(&buf, node, result); err != nil {
				t.Fatalf("PrettyPrintResult() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("PrettyPrintResult() output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

func TestDeeplyNestedObject(t *testing.T) {
	// Test deeply nested object structure (3 levels)
	data := []byte{0x01, 0x02, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	node, err := ParseExpression("<bHiQ | {0 -> a, level1: {1 -> b, level2: {2 -> c, 3 -> d}}}")
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}

	result, err := node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	obj, ok := result.(*Object)
	if !ok {
		t.Fatalf("Expected *Object, got %T", result)
	}

	// Verify structure: {a, level1: {b, level2: {c, d}}}
	if len(obj.Fields) != 2 {
		t.Fatalf("Top level fields = %d, want 2", len(obj.Fields))
	}

	// Check 'a' field
	if obj.Fields[0].Name != "a" || obj.Fields[0].Value != int8(1) {
		t.Errorf("Field 'a' = %+v, want int8(1)", obj.Fields[0])
	}

	// Check level1
	level1, ok := obj.Fields[1].Value.(*Object)
	if !ok {
		t.Fatalf("Expected level1 *Object, got %T", obj.Fields[1].Value)
	}
	if len(level1.Fields) != 2 {
		t.Fatalf("Level1 fields = %d, want 2", len(level1.Fields))
	}
	if level1.Fields[0].Name != "b" || level1.Fields[0].Value != uint16(2) {
		t.Errorf("Field 'b' = %+v, want uint16(2)", level1.Fields[0])
	}

	// Check level2
	level2, ok := level1.Fields[1].Value.(*Object)
	if !ok {
		t.Fatalf("Expected level2 *Object, got %T", level1.Fields[1].Value)
	}
	if len(level2.Fields) != 2 {
		t.Fatalf("Level2 fields = %d, want 2", len(level2.Fields))
	}
	if level2.Fields[0].Name != "c" || level2.Fields[0].Value != int32(3) {
		t.Errorf("Field 'c' = %+v, want int32(3)", level2.Fields[0])
	}
	if level2.Fields[1].Name != "d" || level2.Fields[1].Value != uint64(4) {
		t.Errorf("Field 'd' = %+v, want uint64(4)", level2.Fields[1])
	}
}

// Tests for null-terminated string support

func TestParseWithString(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantCodes []rune
		wantErr   bool
	}{
		{
			name:      "single string",
			format:    "s",
			wantCodes: []rune{'s'},
		},
		{
			name:      "multiple strings",
			format:    "ss",
			wantCodes: []rune{'s', 's'},
		},
		{
			name:      "mixed with binary",
			format:    "<BsH",
			wantCodes: []rune{'B', 's', 'H'},
		},
		{
			name:      "string at end",
			format:    "Bs",
			wantCodes: []rune{'B', 's'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(expr.Formats) != len(tt.wantCodes) {
				t.Errorf("Parse() formats len = %d, want %d", len(expr.Formats), len(tt.wantCodes))
				return
			}

			for i, fc := range expr.Formats {
				if fc.Code != tt.wantCodes[i] {
					t.Errorf("Parse() Formats[%d].Code = %c, want %c", i, fc.Code, tt.wantCodes[i])
				}
			}
		})
	}
}

func TestReadNullTerminatedString(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{
			name: "simple string",
			data: []byte{'h', 'e', 'l', 'l', 'o', 0},
			want: "hello",
		},
		{
			name:    "empty string",
			data:    []byte{0},
			wantErr: true,
		},
		{
			name: "unicode string",
			data: append([]byte("你好"), 0),
			want: "你好",
		},
		{
			name: "string with special chars",
			data: []byte{'h', 'i', ' ', '!', '@', '#', 0},
			want: "hi !@#",
		},
		{
			name: "no null terminator (EOF)",
			data: []byte{'h', 'i'},
			want: "hi",
		},
		{
			name: "string with tab",
			data: []byte{'h', 'i', '\t', 0},
			want: "hi\t",
		},
		{
			name: "string with newline",
			data: []byte{'h', 'i', '\n', 0},
			want: "hi\n",
		},
		// Non-printable character error cases
		{
			name:    "non-printable control char 0x01",
			data:    []byte{'h', 'i', 0x01, 0},
			wantErr: true,
		},
		{
			name:    "non-printable control char 0x7f (DEL)",
			data:    []byte{'h', 'i', 0x7f, 0},
			wantErr: true,
		},
		{
			name:    "non-printable at start",
			data:    []byte{0x02, 'h', 'i', 0},
			wantErr: true,
		},
		{
			name:    "non-printable bell char",
			data:    []byte{'h', 'i', 0x07, 0},
			wantErr: true,
		},
		{
			name:    "non-printable without null (EOF)",
			data:    []byte{'h', 0x01},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readNullTerminatedString(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("readNullTerminatedString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("readNullTerminatedString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpr_ReadString(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		data    []byte
		want    []any
		wantErr bool
	}{
		{
			name:   "single string",
			format: "s",
			data:   []byte{'h', 'e', 'l', 'l', 'o', 0},
			want:   []any{"hello"},
		},
		{
			name:   "two strings",
			format: "ss",
			data:   []byte{'h', 'i', 0, 'b', 'y', 'e', 0},
			want:   []any{"hi", "bye"},
		},
		{
			name:   "mixed binary and string",
			format: "<BsH",
			data:   []byte{0x01, 'h', 'i', 0, 0x02, 0x03},
			want:   []any{uint8(1), "hi", uint16(0x0302)},
		},
		{
			name:   "string then binary",
			format: "sB",
			data:   []byte{'x', 0, 0xFF},
			want:   []any{"x", uint8(255)},
		},
		{
			name:    "empty string",
			format:  "s",
			data:    []byte{0},
			wantErr: true,
		},
		// Non-printable character error cases
		{
			name:    "non-printable character in string",
			format:  "s",
			data:    []byte{'h', 'i', 0x01, 0},
			wantErr: true,
		},
		{
			name:    "non-printable in mixed format bs",
			format:  "<Bs",
			data:    []byte{0x01, 'h', 0x02, 0}, // 0x01 byte is valid, 0x02 in string is not
			wantErr: true,
		},
		{
			name:   "byte with any value followed by valid string",
			format: "<Bs",
			data:   []byte{0x01, 'h', 'i', 0}, // 0x01 is just a byte, "hi" is valid string
			want:   []any{uint8(1), "hi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.format)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := expr.Read(bytes.NewReader(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("Read() len = %v, want %v", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Read() [%d] = %v (%T), want %v (%T)", i, got[i], got[i], tt.want[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseExpressionWithString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple string",
			input:   "s",
			wantErr: false,
		},
		{
			name:    "string with pipe",
			input:   "Bs | {0 -> version, 1 -> name}",
			wantErr: false,
		},
		{
			name:    "multiple strings with object",
			input:   "ss | {0 -> first, 1 -> second}",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && node == nil {
				t.Errorf("ParseExpression() returned nil node without error")
			}
		})
	}
}

func TestStringPipeEval(t *testing.T) {
	// Test string with pipe to object
	data := []byte{0x01, 'h', 'e', 'l', 'l', 'o', 0, 0x02, 0x03}

	node, err := ParseExpression("<BsH | {0 -> version, 1 -> name, 2 -> flags}")
	if err != nil {
		t.Fatalf("ParseExpression() error = %v", err)
	}

	result, err := node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	obj, ok := result.(*Object)
	if !ok {
		t.Fatalf("Eval() returned %T, want *Object", result)
	}

	if len(obj.Fields) != 3 {
		t.Fatalf("Fields len = %d, want 3", len(obj.Fields))
	}

	// Check version field
	if obj.Fields[0].Name != "version" || obj.Fields[0].Value != uint8(1) {
		t.Errorf("Fields[0] = %+v, want {version, 1}", obj.Fields[0])
	}

	// Check name field (string)
	if obj.Fields[1].Name != "name" || obj.Fields[1].Value != "hello" {
		t.Errorf("Fields[1] = %+v, want {name, hello}", obj.Fields[1])
	}

	// Check flags field
	if obj.Fields[2].Name != "flags" || obj.Fields[2].Value != uint16(0x0302) {
		t.Errorf("Fields[2] = %+v, want {flags, 770}", obj.Fields[2])
	}
}

func TestPrettyPrintString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		data     []byte
		contains []string
	}{
		{
			name:  "single string",
			input: "s",
			data:  []byte{'h', 'i', 0},
			contains: []string{
				"Name", "Code", "Type", "Value", "Hex",
				"0", "s", "string", "hi", "68 69",
			},
		},
		{
			name:  "string with binary",
			input: "<BsH | {0 -> ver, 1 -> name, 2 -> id}",
			data:  []byte{0x01, 'a', 'b', 0, 0x02, 0x00},
			contains: []string{
				"ver", "B", "uint8", "1",
				"name", "s", "string", "ab", "61 62",
				"id", "H", "uint16", "2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if err != nil {
				t.Fatalf("ParseExpression() error = %v", err)
			}

			result, err := node.Eval(bytes.NewReader(tt.data), nil)
			if err != nil {
				t.Fatalf("Eval() error = %v", err)
			}

			var buf bytes.Buffer
			if err := PrettyPrintResult(&buf, node, result); err != nil {
				t.Fatalf("PrettyPrintResult() error = %v", err)
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !bytes.Contains([]byte(output), []byte(want)) {
					t.Errorf("PrettyPrintResult() output missing %q\nGot:\n%s", want, output)
				}
			}
		})
	}
}

// === Write functionality tests ===

func TestTokenizerString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []Token
	}{
		{
			name:  "simple string",
			input: `"hello"`,
			tokens: []Token{
				{Type: TokenString, Value: "hello"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "string with escape newline",
			input: `"hello\nworld"`,
			tokens: []Token{
				{Type: TokenString, Value: "hello\nworld"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "string with escape tab",
			input: `"hello\tworld"`,
			tokens: []Token{
				{Type: TokenString, Value: "hello\tworld"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "string with escape quote",
			input: `"say \"hello\""`,
			tokens: []Token{
				{Type: TokenString, Value: `say "hello"`},
				{Type: TokenEOF},
			},
		},
		{
			name:  "string with escape backslash",
			input: `"path\\to\\file"`,
			tokens: []Token{
				{Type: TokenString, Value: `path\to\file`},
				{Type: TokenEOF},
			},
		},
		{
			name:  "write function call",
			input: `write("output.bin")`,
			tokens: []Token{
				{Type: TokenIdent, Value: "write"},
				{Type: TokenLParen, Value: "("},
				{Type: TokenString, Value: "output.bin"},
				{Type: TokenRParen, Value: ")"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "full expression with write",
			input: `<bH | write("out.bin")`,
			tokens: []Token{
				{Type: TokenOrder, Value: "<"},
				{Type: TokenFormat, Value: "b"},
				{Type: TokenFormat, Value: "H"},
				{Type: TokenPipe, Value: "|"},
				{Type: TokenIdent, Value: "write"},
				{Type: TokenLParen, Value: "("},
				{Type: TokenString, Value: "out.bin"},
				{Type: TokenRParen, Value: ")"},
				{Type: TokenEOF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewTokenizer(tt.input)
			for i, want := range tt.tokens {
				got, err := tokenizer.Next()
				if err != nil {
					t.Fatalf("Next() error = %v at token %d", err, i)
				}
				if got.Type != want.Type {
					t.Errorf("Token[%d].Type = %v, want %v", i, got.Type, want.Type)
				}
				if want.Value != "" && got.Value != want.Value {
					t.Errorf("Token[%d].Value = %q, want %q", i, got.Value, want.Value)
				}
			}
		})
	}
}

func TestTokenizerStringErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unterminated string",
			input: `"hello`,
		},
		{
			name:  "unknown escape sequence",
			input: `"hello\x"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewTokenizer(tt.input)
			_, err := tokenizer.Next()
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestParseExpressionWrite(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple write",
			input:   `<bH | write("output.bin")`,
			wantErr: false,
		},
		{
			name:    "write with object",
			input:   `<bH | {0 -> key, 1 -> value} | write("output.bin")`,
			wantErr: false,
		},
		{
			name:    "write missing path",
			input:   `<bH | write()`,
			wantErr: true,
		},
		{
			name:    "write non-string path",
			input:   `<bH | write(123)`,
			wantErr: true,
		},
		{
			name:    "write missing closing paren",
			input:   `<bH | write("output.bin"`,
			wantErr: true,
		},
		{
			name:    "write missing opening paren",
			input:   `<bH | write "output.bin")`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := ParseExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && node == nil {
				t.Error("ParseExpression() returned nil node without error")
			}
		})
	}
}

func TestParseExpressionWriteAST(t *testing.T) {
	// Test that write parsing produces correct AST structure
	t.Run("simple write", func(t *testing.T) {
		node, err := ParseExpression(`<bH | write("output.bin")`)
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		pipeNode, ok := node.(*PipeNode)
		if !ok {
			t.Fatalf("Expected *PipeNode, got %T", node)
		}

		writeNode, ok := pipeNode.Right.(*WriteNode)
		if !ok {
			t.Fatalf("Expected *WriteNode on right, got %T", pipeNode.Right)
		}

		if writeNode.Path != "output.bin" {
			t.Errorf("WriteNode.Path = %q, want %q", writeNode.Path, "output.bin")
		}
	})

	t.Run("chained write", func(t *testing.T) {
		node, err := ParseExpression(`<bH | {0 -> a, 1 -> b} | write("out.bin")`)
		if err != nil {
			t.Fatalf("ParseExpression() error = %v", err)
		}

		// Should be: PipeNode(PipeNode(FormatNode, ObjectNode), WriteNode)
		outerPipe, ok := node.(*PipeNode)
		if !ok {
			t.Fatalf("Expected outer *PipeNode, got %T", node)
		}

		innerPipe, ok := outerPipe.Left.(*PipeNode)
		if !ok {
			t.Fatalf("Expected inner *PipeNode, got %T", outerPipe.Left)
		}

		_, ok = innerPipe.Left.(*FormatNode)
		if !ok {
			t.Fatalf("Expected *FormatNode, got %T", innerPipe.Left)
		}

		_, ok = innerPipe.Right.(*ObjectNode)
		if !ok {
			t.Fatalf("Expected *ObjectNode, got %T", innerPipe.Right)
		}

		writeNode, ok := outerPipe.Right.(*WriteNode)
		if !ok {
			t.Fatalf("Expected *WriteNode, got %T", outerPipe.Right)
		}

		if writeNode.Path != "out.bin" {
			t.Errorf("WriteNode.Path = %q, want %q", writeNode.Path, "out.bin")
		}
	})
}

func TestEncodeValue(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		order   binary.ByteOrder
		want    []byte
		wantErr bool
	}{
		{
			name:  "int8",
			val:   int8(-1),
			order: binary.LittleEndian,
			want:  []byte{0xFF},
		},
		{
			name:  "uint8",
			val:   uint8(255),
			order: binary.LittleEndian,
			want:  []byte{0xFF},
		},
		{
			name:  "int16 little endian",
			val:   int16(0x0201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02},
		},
		{
			name:  "int16 big endian",
			val:   int16(0x0201),
			order: binary.BigEndian,
			want:  []byte{0x02, 0x01},
		},
		{
			name:  "uint16 little endian",
			val:   uint16(0x0201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02},
		},
		{
			name:  "int32 little endian",
			val:   int32(0x04030201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:  "int32 big endian",
			val:   int32(0x04030201),
			order: binary.BigEndian,
			want:  []byte{0x04, 0x03, 0x02, 0x01},
		},
		{
			name:  "uint32",
			val:   uint32(0x04030201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:  "int64 little endian",
			val:   int64(0x0807060504030201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name:  "uint64",
			val:   uint64(0x0807060504030201),
			order: binary.LittleEndian,
			want:  []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name:  "string",
			val:   "hi",
			order: binary.LittleEndian,
			want:  []byte{'h', 'i', 0},
		},
		{
			name:  "empty string",
			val:   "",
			order: binary.LittleEndian,
			want:  []byte{0},
		},
		{
			name:  "[]uint8",
			val:   []uint8{1, 2, 3, 4},
			order: binary.LittleEndian,
			want:  []byte{1, 2, 3, 4},
		},
		{
			name:  "[]int8",
			val:   []int8{-1, 0, 1},
			order: binary.LittleEndian,
			want:  []byte{0xFF, 0x00, 0x01},
		},
		{
			name:  "[]uint16 little endian",
			val:   []uint16{0x0102, 0x0304},
			order: binary.LittleEndian,
			want:  []byte{0x02, 0x01, 0x04, 0x03},
		},
		{
			name:  "[]uint16 big endian",
			val:   []uint16{0x0102, 0x0304},
			order: binary.BigEndian,
			want:  []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:    "unsupported type",
			val:     struct{}{},
			order:   binary.LittleEndian,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := encodeValue(&buf, tt.val, tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("encodeValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("encodeValue() = %v, want %v", buf.Bytes(), tt.want)
			}
		})
	}
}

func TestEncodeObject(t *testing.T) {
	obj := &Object{
		Fields: []ObjectField{
			{Name: "a", Value: int8(-1)},
			{Name: "b", Value: uint16(0x0201)},
		},
	}

	var buf bytes.Buffer
	err := encodeValue(&buf, obj, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encodeValue() error = %v", err)
	}

	want := []byte{0xFF, 0x01, 0x02}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("encodeValue() = %v, want %v", buf.Bytes(), want)
	}
}

func TestWriteNodeEval(t *testing.T) {
	// Create temp file for testing
	tmpFile, err := os.CreateTemp("", "bq-test-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Test writing binary data
	node, err := ParseExpression(fmt.Sprintf(`<bH | write("%s")`, tmpPath))
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}

	data := []byte{0xFF, 0x01, 0x02}
	_, err = node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	// Verify written content
	written, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("Written data = %v, want %v", written, data)
	}
}

func TestWriteWithObject(t *testing.T) {
	// Test that writing through an object preserves data
	tmpFile, err := os.CreateTemp("", "bq-test-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	input := fmt.Sprintf(`<bH | {0 -> key, 1 -> value} | write("%s")`, tmpPath)
	node, err := ParseExpression(input)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}

	data := []byte{0xFF, 0x01, 0x02}
	_, err = node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	// Object writing should preserve original binary data
	written, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("Written data = %v, want %v", written, data)
	}
}

func TestWriteByteOrderPropagation(t *testing.T) {
	tests := []struct {
		name   string
		format string
		data   []byte
		want   []byte
	}{
		{
			name:   "little endian",
			format: "<H",
			data:   []byte{0x01, 0x02},
			want:   []byte{0x01, 0x02},
		},
		{
			name:   "big endian",
			format: ">H",
			data:   []byte{0x01, 0x02},
			want:   []byte{0x01, 0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "bq-test-*.bin")
			if err != nil {
				t.Fatal(err)
			}
			tmpPath := tmpFile.Name()
			_ = tmpFile.Close()
			defer func() { _ = os.Remove(tmpPath) }()

			input := fmt.Sprintf(`%s | write("%s")`, tt.format, tmpPath)
			node, err := ParseExpression(input)
			if err != nil {
				t.Fatalf("ParseExpression error: %v", err)
			}

			_, err = node.Eval(bytes.NewReader(tt.data), nil)
			if err != nil {
				t.Fatalf("Eval error: %v", err)
			}

			written, err := os.ReadFile(tmpPath)
			if err != nil {
				t.Fatalf("ReadFile error: %v", err)
			}
			if !bytes.Equal(written, tt.want) {
				t.Errorf("Written data = %v, want %v", written, tt.want)
			}
		})
	}
}

func TestWriteWithArray(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bq-test-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	input := fmt.Sprintf(`<4B | write("%s")`, tmpPath)
	node, err := ParseExpression(input)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}

	data := []byte{0x01, 0x02, 0x03, 0x04}
	_, err = node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	written, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("Written data = %v, want %v", written, data)
	}
}

func TestWriteWithString(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bq-test-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	input := fmt.Sprintf(`<Bs | write("%s")`, tmpPath)
	node, err := ParseExpression(input)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}

	data := []byte{0x01, 'h', 'i', 0}
	_, err = node.Eval(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	written, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("Written data = %v, want %v", written, data)
	}
}
