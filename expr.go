package bq

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/rs/zerolog/log"
)

// Node is the interface for all AST nodes in the expression tree.
type Node interface {
	// Eval evaluates the node with the given input values and reader.
	// For format nodes, it reads from the reader.
	// For object nodes, it transforms the input values.
	Eval(r io.Reader, values []any) (any, error)
}

// FormatNode represents binary format parsing (wraps existing Expr logic).
type FormatNode struct {
	*Expr
}

// Eval reads binary data from the reader according to the format codes.
func (n *FormatNode) Eval(r io.Reader, _ []any) (any, error) {
	return n.Read(r)
}

// PipeNode chains two nodes together, passing output from left to right.
type PipeNode struct {
	Left  Node // produces []any
	Right Node // consumes []any
}

// Eval evaluates the left node, then passes its result to the right node.
func (n *PipeNode) Eval(r io.Reader, values []any) (any, error) {
	leftResult, err := n.Left.Eval(r, values)
	if err != nil {
		return nil, err
	}

	// Left result should be []any for piping to right
	leftValues, ok := leftResult.([]any)
	if !ok {
		return nil, fmt.Errorf("pipe left side must produce []any, got %T", leftResult)
	}

	return n.Right.Eval(r, leftValues)
}

// FieldDef defines a single field in an object with index mapping or nested object.
type FieldDef struct {
	Index  int         // index into the input values (ignored if Nested is set)
	Name   string      // field name in the output object
	Nested *ObjectNode // nested object definition (nil for regular index field)
}

// ObjectNode creates named fields from indexed values.
type ObjectNode struct {
	Fields []FieldDef // ordered list of field definitions
}

// Eval transforms the input values into an Object with named fields.
func (n *ObjectNode) Eval(_ io.Reader, values []any) (any, error) {
	obj := &Object{
		Fields: make([]ObjectField, 0, len(n.Fields)),
	}

	for _, fd := range n.Fields {
		if fd.Nested != nil {
			// Nested object: recursively evaluate
			nestedResult, err := fd.Nested.Eval(nil, values)
			if err != nil {
				return nil, fmt.Errorf("nested field %q: %w", fd.Name, err)
			}
			obj.Fields = append(obj.Fields, ObjectField{
				Name:  fd.Name,
				Value: nestedResult,
			})
		} else {
			// Regular index field
			if fd.Index < 0 || fd.Index >= len(values) {
				return nil, fmt.Errorf("field %q: index %d out of range (have %d values)", fd.Name, fd.Index, len(values))
			}
			obj.Fields = append(obj.Fields, ObjectField{
				Name:  fd.Name,
				Value: values[fd.Index],
			})
		}
	}

	return obj, nil
}

// ObjectField represents a single field in an Object result.
type ObjectField struct {
	Name  string // field name
	Value any    // field value
}

// Object represents the result of object construction.
type Object struct {
	Fields []ObjectField
}

// TokenType represents the type of a token in the expression.
type TokenType int

const (
	TokenEOF    TokenType = iota // end of input
	TokenPipe                    // |
	TokenLBrace                  // {
	TokenRBrace                  // }
	TokenLParen                  // (
	TokenRParen                  // )
	TokenColon                   // :
	TokenArrow                   // ->
	TokenComma                   // ,
	TokenNumber                  // integer literal (for index)
	TokenIdent                   // identifier (for field name or function)
	TokenFormat                  // format code (b, B, h, H, i, I, q, Q)
	TokenOrder                   // byte order prefix (<, >, @)
)

// Token represents a single token in the expression.
type Token struct {
	Type  TokenType
	Value string // the literal value of the token
	Pos   int    // position in the input string
}

// Tokenizer breaks an expression string into tokens.
type Tokenizer struct {
	input []rune
	pos   int
}

// NewTokenizer creates a new tokenizer for the given input.
func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{
		input: []rune(input),
		pos:   0,
	}
}

// Next returns the next token from the input.
func (t *Tokenizer) Next() (Token, error) {
	t.skipWhitespace()

	if t.pos >= len(t.input) {
		return Token{Type: TokenEOF, Pos: t.pos}, nil
	}

	startPos := t.pos
	ch := t.input[t.pos]

	// Single character tokens
	switch ch {
	case '|':
		t.pos++
		return Token{Type: TokenPipe, Value: "|", Pos: startPos}, nil
	case '{':
		t.pos++
		return Token{Type: TokenLBrace, Value: "{", Pos: startPos}, nil
	case '}':
		t.pos++
		return Token{Type: TokenRBrace, Value: "}", Pos: startPos}, nil
	case '(':
		t.pos++
		return Token{Type: TokenLParen, Value: "(", Pos: startPos}, nil
	case ')':
		t.pos++
		return Token{Type: TokenRParen, Value: ")", Pos: startPos}, nil
	case ':':
		t.pos++
		return Token{Type: TokenColon, Value: ":", Pos: startPos}, nil
	case ',':
		t.pos++
		return Token{Type: TokenComma, Value: ",", Pos: startPos}, nil
	case '<', '>', '@':
		t.pos++
		return Token{Type: TokenOrder, Value: string(ch), Pos: startPos}, nil
	}

	// Arrow operator ->
	if ch == '-' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '>' {
		t.pos += 2
		return Token{Type: TokenArrow, Value: "->", Pos: startPos}, nil
	}

	// Number (for index)
	if isDigit(ch) {
		return t.scanNumber(startPos)
	}

	// Format codes (single character) - only if not followed by non-format-code letters
	// This distinguishes format codes like 'b' from identifiers like 'bar'
	// Examples: 'bH' -> two format codes, 'b4B' -> format b, number 4, format B, 'bar' -> identifier
	if _, ok := formatCodeInfo[ch]; ok {
		nextPos := t.pos + 1
		if nextPos >= len(t.input) {
			// End of input - it's a format code
			t.pos++
			return Token{Type: TokenFormat, Value: string(ch), Pos: startPos}, nil
		}
		nextCh := t.input[nextPos]
		// If next char is a format code, digit, or not alphanumeric, treat current as format code
		// Only treat as identifier start if followed by non-format-code letter
		_, nextIsFormat := formatCodeInfo[nextCh]
		if !isAlphanumeric(nextCh) || nextIsFormat || isDigit(nextCh) {
			t.pos++
			return Token{Type: TokenFormat, Value: string(ch), Pos: startPos}, nil
		}
	}

	// Identifier (for field names)
	if isLetter(ch) || ch == '_' {
		return t.scanIdent(startPos)
	}

	return Token{}, fmt.Errorf("unexpected character %q at position %d", ch, startPos)
}

// Peek returns the next token without consuming it.
func (t *Tokenizer) Peek() (Token, error) {
	savedPos := t.pos
	tok, err := t.Next()
	t.pos = savedPos
	return tok, err
}

// skipWhitespace advances past any whitespace characters.
func (t *Tokenizer) skipWhitespace() {
	for t.pos < len(t.input) && isWhitespace(t.input[t.pos]) {
		t.pos++
	}
}

// scanNumber scans a number token.
func (t *Tokenizer) scanNumber(startPos int) (Token, error) {
	for t.pos < len(t.input) && isDigit(t.input[t.pos]) {
		t.pos++
	}
	return Token{Type: TokenNumber, Value: string(t.input[startPos:t.pos]), Pos: startPos}, nil
}

// scanIdent scans an identifier token.
func (t *Tokenizer) scanIdent(startPos int) (Token, error) {
	for t.pos < len(t.input) && (isLetter(t.input[t.pos]) || isDigit(t.input[t.pos]) || t.input[t.pos] == '_') {
		t.pos++
	}
	return Token{Type: TokenIdent, Value: string(t.input[startPos:t.pos]), Pos: startPos}, nil
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isAlphanumeric(ch rune) bool {
	return isLetter(ch) || isDigit(ch) || ch == '_'
}

// Parser uses the tokenizer to build an AST from an expression string.
type Parser struct {
	tokenizer *Tokenizer
	current   Token
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	return &Parser{
		tokenizer: NewTokenizer(input),
	}
}

// ParseExpression parses an expression and returns the root AST node.
// Grammar:
//
//	Expression  → Pipe
//	Pipe        → Primary ('|' Object)?
//	Primary     → FunctionCall | FormatExpr
//	FunctionCall→ IDENT '(' FormatExpr ')'
//	FormatExpr  → ByteOrder? (Count? FormatCode)+
//	Object      → '{' FieldList '}'
//	FieldList   → FieldItem (',' FieldItem)*
//	FieldItem   → IndexField | NestedField
//	IndexField  → NUMBER '->' IDENTIFIER
//	NestedField → IDENTIFIER ':' Object
func ParseExpression(input string) (Node, error) {
	p := NewParser(input)
	if err := p.advance(); err != nil {
		return nil, err
	}
	return p.parseExpression()
}

// advance moves to the next token.
func (p *Parser) advance() error {
	tok, err := p.tokenizer.Next()
	if err != nil {
		return err
	}
	p.current = tok
	return nil
}

// parseExpression parses the top-level expression (Pipe rule).
func (p *Parser) parseExpression() (Node, error) {
	return p.parsePipe()
}

// parsePipe parses: Primary ('|' Object)?
func (p *Parser) parsePipe() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	// Check for pipe operator
	if p.current.Type == TokenPipe {
		if err := p.advance(); err != nil {
			return nil, err
		}

		right, err := p.parseObject()
		if err != nil {
			return nil, err
		}

		return &PipeNode{Left: left, Right: right}, nil
	}

	return left, nil
}

// parsePrimary parses: FunctionCall | FormatExpr
func (p *Parser) parsePrimary() (Node, error) {
	// Check for function call: IDENT '(' ... ')'
	if p.current.Type == TokenIdent {
		// Peek ahead to see if next token is '('
		nextTok, err := p.tokenizer.Peek()
		if err != nil {
			return nil, err
		}
		if nextTok.Type == TokenLParen {
			return p.parseFunctionCall()
		}
	}
	return p.parseFormatExpr()
}

// parseFunctionCall parses: IDENT '(' FormatExpr ')'
// Currently only supports the 'parse' function.
func (p *Parser) parseFunctionCall() (Node, error) {
	funcName := p.current.Value
	if err := p.advance(); err != nil {
		return nil, err
	}

	// Consume '('
	if p.current.Type != TokenLParen {
		return nil, fmt.Errorf("expected '(' after function name at position %d", p.current.Pos)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	// Parse the argument (format expression)
	arg, err := p.parseFormatExpr()
	if err != nil {
		return nil, err
	}

	// Consume ')'
	if p.current.Type != TokenRParen {
		return nil, fmt.Errorf("expected ')' after function argument at position %d", p.current.Pos)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	// Handle known functions
	switch funcName {
	case "parse":
		// parse() just returns the format expression as-is
		return arg, nil
	default:
		return nil, fmt.Errorf("unknown function %q at position %d", funcName, p.current.Pos)
	}
}

// parseFormatExpr parses: ByteOrder? (Count? FormatCode)+
// Count is an optional digit prefix for arrays, e.g., 4B means 4 unsigned chars.
func (p *Parser) parseFormatExpr() (Node, error) {
	expr := &Expr{
		Order:   NativeOrder,
		Formats: make([]FormatCode, 0),
	}

	// Check for byte order prefix
	if p.current.Type == TokenOrder {
		switch p.current.Value {
		case "<":
			expr.Order = LittleEndian
		case ">":
			expr.Order = BigEndian
		case "@":
			expr.Order = NativeOrder
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}

	// Parse format codes with optional count prefix
	for p.current.Type == TokenFormat || p.current.Type == TokenNumber {
		count := 1

		// Check for count prefix (e.g., 4B means 4 unsigned chars)
		if p.current.Type == TokenNumber {
			var err error
			count, err = strconv.Atoi(p.current.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid count %q: %w", p.current.Value, err)
			}
			if count < 1 {
				return nil, fmt.Errorf("count must be at least 1, got %d", count)
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			// After a number, we must have a format code
			if p.current.Type != TokenFormat {
				return nil, fmt.Errorf("expected format code after count at position %d", p.current.Pos)
			}
		}

		code := rune(p.current.Value[0])
		info := formatCodeInfo[code]
		expr.Formats = append(expr.Formats, FormatCode{
			Code:   code,
			Size:   info.size,
			Signed: info.signed,
			Count:  count,
		})
		if err := p.advance(); err != nil {
			return nil, err
		}
	}

	if len(expr.Formats) == 0 {
		return nil, fmt.Errorf("expected format codes at position %d", p.current.Pos)
	}

	return &FormatNode{Expr: expr}, nil
}

// parseObject parses: '{' FieldList '}'
func (p *Parser) parseObject() (Node, error) {
	if p.current.Type != TokenLBrace {
		return nil, fmt.Errorf("expected '{' at position %d, got %q", p.current.Pos, p.current.Value)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	fields, err := p.parseFieldList()
	if err != nil {
		return nil, err
	}

	if p.current.Type != TokenRBrace {
		return nil, fmt.Errorf("expected '}' at position %d, got %q", p.current.Pos, p.current.Value)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}

	return &ObjectNode{Fields: fields}, nil
}

// parseFieldList parses: FieldItem (',' FieldItem)*
func (p *Parser) parseFieldList() ([]FieldDef, error) {
	fields := make([]FieldDef, 0)

	// Parse first field
	fd, err := p.parseFieldItem()
	if err != nil {
		return nil, err
	}
	fields = append(fields, fd)

	// Parse remaining fields
	for p.current.Type == TokenComma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		fd, err := p.parseFieldItem()
		if err != nil {
			return nil, err
		}
		fields = append(fields, fd)
	}

	return fields, nil
}

// parseFieldItem parses: IndexField | NestedField
// IndexField  → NUMBER '->' IDENTIFIER
// NestedField → IDENTIFIER ':' Object
// Note: Nested field names can also be format code characters.
func (p *Parser) parseFieldItem() (FieldDef, error) {
	// Check if it's a nested field (starts with identifier or format code followed by ':')
	if p.current.Type == TokenIdent || p.current.Type == TokenFormat {
		// Peek to see if next token is ':'
		nextTok, err := p.tokenizer.Peek()
		if err != nil {
			return FieldDef{}, err
		}
		if nextTok.Type == TokenColon {
			return p.parseNestedField()
		}
	}

	// Otherwise it's an index field
	return p.parseIndexField()
}

// parseIndexField parses: NUMBER '->' IDENTIFIER
// Note: Field names can also be format code characters (b, B, h, H, i, I, q, Q),
// which the tokenizer may classify as TokenFormat instead of TokenIdent.
func (p *Parser) parseIndexField() (FieldDef, error) {
	if p.current.Type != TokenNumber {
		return FieldDef{}, fmt.Errorf("expected index number at position %d, got %q", p.current.Pos, p.current.Value)
	}

	index, err := strconv.Atoi(p.current.Value)
	if err != nil {
		return FieldDef{}, fmt.Errorf("invalid index number %q: %w", p.current.Value, err)
	}
	if err := p.advance(); err != nil {
		return FieldDef{}, err
	}

	if p.current.Type != TokenArrow {
		return FieldDef{}, fmt.Errorf("expected '->' at position %d, got %q", p.current.Pos, p.current.Value)
	}
	if err := p.advance(); err != nil {
		return FieldDef{}, err
	}

	// Accept both TokenIdent and TokenFormat as field names
	// (format code characters like 'b', 'H' can be valid field names)
	if p.current.Type != TokenIdent && p.current.Type != TokenFormat {
		return FieldDef{}, fmt.Errorf("expected field name at position %d, got %q", p.current.Pos, p.current.Value)
	}

	name := p.current.Value
	if err := p.advance(); err != nil {
		return FieldDef{}, err
	}

	return FieldDef{Index: index, Name: name}, nil
}

// parseNestedField parses: IDENTIFIER ':' Object
// Note: Nested field names can also be format code characters.
func (p *Parser) parseNestedField() (FieldDef, error) {
	// Accept both TokenIdent and TokenFormat as nested field names
	if p.current.Type != TokenIdent && p.current.Type != TokenFormat {
		return FieldDef{}, fmt.Errorf("expected nested field name at position %d, got %q", p.current.Pos, p.current.Value)
	}

	name := p.current.Value
	if err := p.advance(); err != nil {
		return FieldDef{}, err
	}

	if p.current.Type != TokenColon {
		return FieldDef{}, fmt.Errorf("expected ':' at position %d, got %q", p.current.Pos, p.current.Value)
	}
	if err := p.advance(); err != nil {
		return FieldDef{}, err
	}

	// Parse the nested object
	nestedNode, err := p.parseObject()
	if err != nil {
		return FieldDef{}, err
	}

	nested, ok := nestedNode.(*ObjectNode)
	if !ok {
		return FieldDef{}, fmt.Errorf("expected object for nested field %q", name)
	}

	return FieldDef{Name: name, Nested: nested}, nil
}

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
	// Size is the number of bytes this format code reads (per element).
	Size int
	// Signed indicates whether the value is signed.
	Signed bool
	// Count is the number of elements to read (1 for single value, >1 for array).
	Count int
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

// formatCodeTypeName returns the human-readable type name for each format code.
var formatCodeTypeName = map[rune]string{
	'b': "int8",
	'B': "uint8",
	'h': "int16",
	'H': "uint16",
	'i': "int32",
	'I': "uint32",
	'q': "int64",
	'Q': "uint64",
}

// Parse parses a format string and returns an Expr.
// The format string consists of an optional byte order prefix followed by format codes.
// Byte order prefixes: '<' (little-endian), '>' (big-endian), '@' (native)
// Format codes: b, B, h, H, i, I, q, Q
// Count prefix: optional digit(s) before format code, e.g., 4B means 4 unsigned chars
func Parse(format string) (*Expr, error) {
	if len(format) == 0 {
		return nil, fmt.Errorf("empty format string")
	}

	expr := &Expr{
		Order:   NativeOrder,
		Formats: make([]FormatCode, 0),
	}

	runes := []rune(format)
	i := 0

	// Check for byte order prefix
	switch runes[0] {
	case '<':
		expr.Order = LittleEndian
		i = 1
	case '>':
		expr.Order = BigEndian
		i = 1
	case '@':
		expr.Order = NativeOrder
		i = 1
	}

	// Parse format codes with optional count prefix
	for i < len(runes) {
		count := 1

		// Check for count prefix (digits)
		if isDigit(runes[i]) {
			countStart := i
			for i < len(runes) && isDigit(runes[i]) {
				i++
			}
			var err error
			count, err = strconv.Atoi(string(runes[countStart:i]))
			if err != nil {
				return nil, fmt.Errorf("invalid count: %w", err)
			}
			if count < 1 {
				return nil, fmt.Errorf("count must be at least 1, got %d", count)
			}
		}

		if i >= len(runes) {
			return nil, fmt.Errorf("expected format code after count")
		}

		code := runes[i]
		info, ok := formatCodeInfo[code]
		if !ok {
			return nil, fmt.Errorf("unknown format code: %c", code)
		}

		expr.Formats = append(expr.Formats, FormatCode{
			Code:   code,
			Size:   info.size,
			Signed: info.signed,
			Count:  count,
		})
		i++
	}

	if len(expr.Formats) == 0 {
		return nil, fmt.Errorf("no format codes in expression")
	}

	return expr, nil
}

// Read reads binary data from the reader and returns the parsed values.
// For format codes with Count > 1, returns a typed slice (e.g., []int8 for 4b).
func (e *Expr) Read(r io.Reader) ([]any, error) {
	order := e.binaryOrder()
	values := make([]any, 0, len(e.Formats))

	for _, fc := range e.Formats {
		count := fc.Count
		if count == 0 {
			count = 1 // default for backward compatibility
		}

		if count == 1 {
			// Single value
			buf := make([]byte, fc.Size)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, fmt.Errorf("failed to read %d bytes for format %c: %w", fc.Size, fc.Code, err)
			}
			val, err := fc.decode(buf, order)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		} else {
			// Array of values
			arr, err := fc.decodeArray(r, order, count)
			if err != nil {
				return nil, err
			}
			values = append(values, arr)
		}
	}

	return values, nil
}

// decodeArray reads count elements and returns a typed slice.
func (fc *FormatCode) decodeArray(r io.Reader, order binary.ByteOrder, count int) (any, error) {
	totalSize := fc.Size * count
	buf := make([]byte, totalSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("failed to read %d bytes for %d x format %c: %w", totalSize, count, fc.Code, err)
	}

	switch fc.Code {
	case 'b': // []int8
		arr := make([]int8, count)
		for i := 0; i < count; i++ {
			arr[i] = int8(buf[i])
		}
		return arr, nil
	case 'B': // []uint8
		arr := make([]uint8, count)
		copy(arr, buf)
		return arr, nil
	case 'h': // []int16
		arr := make([]int16, count)
		for i := 0; i < count; i++ {
			arr[i] = int16(order.Uint16(buf[i*2:]))
		}
		return arr, nil
	case 'H': // []uint16
		arr := make([]uint16, count)
		for i := 0; i < count; i++ {
			arr[i] = order.Uint16(buf[i*2:])
		}
		return arr, nil
	case 'i': // []int32
		arr := make([]int32, count)
		for i := 0; i < count; i++ {
			arr[i] = int32(order.Uint32(buf[i*4:]))
		}
		return arr, nil
	case 'I': // []uint32
		arr := make([]uint32, count)
		for i := 0; i < count; i++ {
			arr[i] = order.Uint32(buf[i*4:])
		}
		return arr, nil
	case 'q': // []int64
		arr := make([]int64, count)
		for i := 0; i < count; i++ {
			arr[i] = int64(order.Uint64(buf[i*8:]))
		}
		return arr, nil
	case 'Q': // []uint64
		arr := make([]uint64, count)
		for i := 0; i < count; i++ {
			arr[i] = order.Uint64(buf[i*8:])
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unknown format code: %c", fc.Code)
	}
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

// Execute parses the expression, reads from the reader, and outputs the result.
func Execute(format string, r io.Reader, pretty bool) error {
	node, err := ParseExpression(format)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse expression")
		return err
	}

	result, err := node.Eval(r, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to evaluate expression")
		return err
	}

	if pretty {
		return PrettyPrintResult(os.Stdout, node, result)
	}

	log.Info().Any("result", result).Msg("evaluated expression")
	return nil
}

// PrettyPrint outputs the parsed values in a human-readable format.
func PrettyPrint(w io.Writer, expr *Expr, values []any) error {
	// Print header
	if _, err := fmt.Fprintf(w, "%-6s %-6s %-8s %20s %20s\n", "Index", "Code", "Type", "Value", "Hex"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", "--------------------------------------------------------------"); err != nil {
		return err
	}

	for i, val := range values {
		fc := expr.Formats[i]
		typeName := formatCodeTypeName[fc.Code]
		hexStr := formatHex(val)

		if _, err := fmt.Fprintf(w, "%-6d %-6c %-8s %20v %20s\n", i, fc.Code, typeName, val, hexStr); err != nil {
			return err
		}
	}

	return nil
}

// formatHex formats a value as a hexadecimal string.
func formatHex(val any) string {
	switch v := val.(type) {
	case int8:
		return fmt.Sprintf("0x%02x", uint8(v))
	case uint8:
		return fmt.Sprintf("0x%02x", v)
	case int16:
		return fmt.Sprintf("0x%04x", uint16(v))
	case uint16:
		return fmt.Sprintf("0x%04x", v)
	case int32:
		return fmt.Sprintf("0x%08x", uint32(v))
	case uint32:
		return fmt.Sprintf("0x%08x", v)
	case int64:
		return fmt.Sprintf("0x%016x", uint64(v))
	case uint64:
		return fmt.Sprintf("0x%016x", v)
	// Array types
	case []int8:
		return formatHexArray(v, func(x int8) string { return fmt.Sprintf("%02x", uint8(x)) })
	case []uint8:
		return formatHexArray(v, func(x uint8) string { return fmt.Sprintf("%02x", x) })
	case []int16:
		return formatHexArray(v, func(x int16) string { return fmt.Sprintf("%04x", uint16(x)) })
	case []uint16:
		return formatHexArray(v, func(x uint16) string { return fmt.Sprintf("%04x", x) })
	case []int32:
		return formatHexArray(v, func(x int32) string { return fmt.Sprintf("%08x", uint32(x)) })
	case []uint32:
		return formatHexArray(v, func(x uint32) string { return fmt.Sprintf("%08x", x) })
	case []int64:
		return formatHexArray(v, func(x int64) string { return fmt.Sprintf("%016x", uint64(x)) })
	case []uint64:
		return formatHexArray(v, func(x uint64) string { return fmt.Sprintf("%016x", x) })
	default:
		return "N/A"
	}
}

// formatHexArray formats an array of values as a hexadecimal string.
func formatHexArray[T any](arr []T, format func(T) string) string {
	if len(arr) == 0 {
		return "[]"
	}
	parts := make([]string, len(arr))
	for i, v := range arr {
		parts[i] = format(v)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, " "))
}

// PrettyPrintResult outputs any evaluation result in a human-readable format.
// It handles both []any (from FormatNode) and *Object (from ObjectNode).
func PrettyPrintResult(w io.Writer, node Node, result any) error {
	// Print header
	if _, err := fmt.Fprintf(w, "%-10s %-6s %-8s %20s %20s\n", "Name", "Code", "Type", "Value", "Hex"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", "--------------------------------------------------------------------"); err != nil {
		return err
	}

	return prettyPrintValue(w, node, result, 0)
}

// prettyPrintValue recursively prints values with indentation for nested objects.
func prettyPrintValue(w io.Writer, node Node, result any, indent int) error {
	indentStr := strings.Repeat("  ", indent)

	switch r := result.(type) {
	case []any:
		// Result from FormatNode - use indices as names
		formatNode, ok := extractFormatNode(node)
		if ok {
			for i, val := range r {
				fc := formatNode.Formats[i]
				typeName := formatCodeTypeName[fc.Code]
				hexStr := formatHex(val)
				valStr := formatValue(val)
				name := fmt.Sprintf("%s%d", indentStr, i)
				if _, err := fmt.Fprintf(w, "%-10s %-6c %-8s %20s %20s\n", name, fc.Code, typeName, valStr, hexStr); err != nil {
					return err
				}
			}
		} else {
			// Fallback if no format info available
			for i, val := range r {
				code, typeName := inferTypeInfo(val)
				hexStr := formatHex(val)
				valStr := formatValue(val)
				name := fmt.Sprintf("%s%d", indentStr, i)
				if _, err := fmt.Fprintf(w, "%-10s %-6c %-8s %20s %20s\n", name, code, typeName, valStr, hexStr); err != nil {
					return err
				}
			}
		}
	case *Object:
		// Result from ObjectNode - use field names
		for _, field := range r.Fields {
			// Check if field value is a nested object
			if nestedObj, ok := field.Value.(*Object); ok {
				// Print nested object header
				name := fmt.Sprintf("%s%s", indentStr, field.Name)
				if _, err := fmt.Fprintf(w, "%-10s %-6s %-8s %20s %20s\n", name, "-", "object", "", ""); err != nil {
					return err
				}
				// Recursively print nested fields
				if err := prettyPrintValue(w, nil, nestedObj, indent+1); err != nil {
					return err
				}
			} else {
				code, typeName := inferTypeInfo(field.Value)
				hexStr := formatHex(field.Value)
				valStr := formatValue(field.Value)
				name := fmt.Sprintf("%s%s", indentStr, field.Name)
				if _, err := fmt.Fprintf(w, "%-10s %-6c %-8s %20s %20s\n", name, code, typeName, valStr, hexStr); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unsupported result type: %T", result)
	}

	return nil
}

// extractFormatNode extracts the FormatNode from a node tree (handles PipeNode).
func extractFormatNode(node Node) (*Expr, bool) {
	switch n := node.(type) {
	case *FormatNode:
		return n.Expr, true
	case *PipeNode:
		return extractFormatNode(n.Left)
	default:
		return nil, false
	}
}

// isArrayValue returns true if the value is an array type.
func isArrayValue(val any) bool {
	switch val.(type) {
	case []int8, []uint8, []int16, []uint16, []int32, []uint32, []int64, []uint64:
		return true
	default:
		return false
	}
}

// formatValue formats a value for the Value column.
// For arrays, returns empty string (only Hex is shown).
// For scalars, returns the value as a string.
func formatValue(val any) string {
	if isArrayValue(val) {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

// inferTypeInfo infers the format code and type name from a value's Go type.
func inferTypeInfo(val any) (rune, string) {
	switch val.(type) {
	case int8:
		return 'b', "int8"
	case uint8:
		return 'B', "uint8"
	case int16:
		return 'h', "int16"
	case uint16:
		return 'H', "uint16"
	case int32:
		return 'i', "int32"
	case uint32:
		return 'I', "uint32"
	case int64:
		return 'q', "int64"
	case uint64:
		return 'Q', "uint64"
	// Array types
	case []int8:
		return 'b', "[]int8"
	case []uint8:
		return 'B', "[]uint8"
	case []int16:
		return 'h', "[]int16"
	case []uint16:
		return 'H', "[]uint16"
	case []int32:
		return 'i', "[]int32"
	case []uint32:
		return 'I', "[]uint32"
	case []int64:
		return 'q', "[]int64"
	case []uint64:
		return 'Q', "[]uint64"
	default:
		return '?', "unknown"
	}
}
