package bq

import (
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"

	"github.com/rs/zerolog/log"
)

// ByteOrder represents the byte order (endianness) for reading binary data.
type ByteOrder int

const (
	// NativeOrder uses the native byte order of the current system.
	NativeOrder ByteOrder = iota
	// LittleEndian uses little-endian byte order.
	LittleEndian
	// BigEndian uses big-endian byte order.
	BigEndian
)

// FormatCode represents a single format specifier in the expression.
type FormatCode struct {
	// Code is the single character format code (b, B, h, H, i, I, q, Q).
	Code rune
	// Size is the number of bytes this format code reads.
	Size int
	// Signed indicates whether the value is signed.
	Signed bool
}

// Expr represents a parsed binary format expression.
type Expr struct {
	// Order is the byte order for reading binary data.
	Order ByteOrder
	// Formats is the list of format codes to apply.
	Formats []FormatCode
}

// formatCodeInfo holds the size and signedness for each format code.
var formatCodeInfo = map[rune]struct {
	size   int
	signed bool
}{
	'b': {1, true},  // signed char
	'B': {1, false}, // unsigned char
	'h': {2, true},  // signed short
	'H': {2, false}, // unsigned short
	'i': {4, true},  // signed int
	'I': {4, false}, // unsigned int
	'q': {8, true},  // signed long
	'Q': {8, false}, // unsigned long
}

// Parse parses a format string and returns an Expr.
// The format string consists of an optional byte order prefix followed by format codes.
// Byte order prefixes: '<' (little-endian), '>' (big-endian), '@' (native)
// Format codes: b, B, h, H, i, I, q, Q
func Parse(format string) (*Expr, error) {
	if len(format) == 0 {
		return nil, fmt.Errorf("empty format string")
	}

	expr := &Expr{
		Order:   NativeOrder,
		Formats: make([]FormatCode, 0),
	}

	runes := []rune(format)
	start := 0

	// Check for byte order prefix
	switch runes[0] {
	case '<':
		expr.Order = LittleEndian
		start = 1
	case '>':
		expr.Order = BigEndian
		start = 1
	case '@':
		expr.Order = NativeOrder
		start = 1
	}

	// Parse format codes
	for i := start; i < len(runes); i++ {
		code := runes[i]
		info, ok := formatCodeInfo[code]
		if !ok {
			return nil, fmt.Errorf("unknown format code: %c", code)
		}

		expr.Formats = append(expr.Formats, FormatCode{
			Code:   code,
			Size:   info.size,
			Signed: info.signed,
		})
	}

	if len(expr.Formats) == 0 {
		return nil, fmt.Errorf("no format codes in expression")
	}

	return expr, nil
}

// Read reads binary data from the reader and returns the parsed values.
func (e *Expr) Read(r io.Reader) ([]any, error) {
	order := e.binaryOrder()
	values := make([]any, 0, len(e.Formats))

	for _, fc := range e.Formats {
		buf := make([]byte, fc.Size)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("failed to read %d bytes for format %c: %w", fc.Size, fc.Code, err)
		}

		val, err := fc.decode(buf, order)
		if err != nil {
			return nil, err
		}
		values = append(values, val)
	}

	return values, nil
}

// binaryOrder returns the binary.ByteOrder for this expression.
func (e *Expr) binaryOrder() binary.ByteOrder {
	switch e.Order {
	case LittleEndian:
		return binary.LittleEndian
	case BigEndian:
		return binary.BigEndian
	default:
		return nativeEndian()
	}
}

// nativeEndian returns the native byte order of the current system.
func nativeEndian() binary.ByteOrder {
	var x uint16 = 0x0102
	if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// decode decodes the bytes into the appropriate type based on the format code.
func (fc *FormatCode) decode(buf []byte, order binary.ByteOrder) (any, error) {
	switch fc.Code {
	case 'b': // signed char
		return int8(buf[0]), nil
	case 'B': // unsigned char
		return buf[0], nil
	case 'h': // signed short
		return int16(order.Uint16(buf)), nil
	case 'H': // unsigned short
		return order.Uint16(buf), nil
	case 'i': // signed int
		return int32(order.Uint32(buf)), nil
	case 'I': // unsigned int
		return order.Uint32(buf), nil
	case 'q': // signed long
		return int64(order.Uint64(buf)), nil
	case 'Q': // unsigned long
		return order.Uint64(buf), nil
	default:
		return nil, fmt.Errorf("unknown format code: %c", fc.Code)
	}
}

// Execute parses the expression, reads from the reader, and logs the result.
func Execute(format string, r io.Reader) error {
	expr, err := Parse(format)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse expression")
		return err
	}

	values, err := expr.Read(r)
	if err != nil {
		log.Error().Err(err).Msg("failed to read binary data")
		return err
	}

	log.Info().Any("values", values).Msg("parsed binary data")
	return nil
}
