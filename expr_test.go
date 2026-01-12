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
