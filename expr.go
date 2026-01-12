package bq

import (
	"fmt"
	"io"

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

// Parse parses a format string and returns an Expr.
// The format string consists of an optional byte order prefix followed by format codes.
// Byte order prefixes: '<' (little-endian), '>' (big-endian), '@' (native)
// Format codes: b, B, h, H, i, I, q, Q
func Parse(format string) (*Expr, error) {
	// TODO: implement parsing
	return nil, fmt.Errorf("not implemented")
}

// Read reads binary data from the reader and returns the parsed values.
func (e *Expr) Read(r io.Reader) ([]any, error) {
	// TODO: implement reading
	return nil, fmt.Errorf("not implemented")
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
