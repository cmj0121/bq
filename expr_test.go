package bq

import (
	"bytes"
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
		{"unknown type", "string", "N/A"},
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
		{"string", '?', "unknown"},
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
