package mdx

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// TokenStream reads and writes MDL tokens.
type TokenStream struct {
	Buffer       string
	Index        int
	Ident        int
	IndentSpaces int
	Precision    float64
}

// NewTokenStream creates a token stream over buffer.
func NewTokenStream(buffer string) *TokenStream {
	return &TokenStream{
		Buffer:       buffer,
		IndentSpaces: 4,
		Precision:    1000000,
	}
}

// Clear resets the stream.
func (s *TokenStream) Clear() {
	s.Buffer = ""
	s.Index = 0
	s.Ident = 0
}

// ReadToken reads the next token, or empty string at EOF.
func (s *TokenStream) ReadToken() string {
	buffer := s.Buffer
	length := len(buffer)
	inComment := false
	inString := false
	token := strings.Builder{}

	for s.Index < length {
		c := buffer[s.Index]
		s.Index++

		if inComment {
			if c == '\n' {
				inComment = false
			}
		} else if inString {
			if c == '\\' {
				token.WriteByte(c)
				if s.Index < length {
					token.WriteByte(buffer[s.Index])
					s.Index++
				}
			} else if c == '\n' {
				token.WriteString("\\n")
			} else if c == '\r' {
				token.WriteString("\\r")
			} else if c == '"' {
				return token.String()
			} else {
				token.WriteByte(c)
			}
		} else if c == ' ' || c == ',' || c == '\t' || c == '\n' || c == ':' || c == '\r' {
			if token.Len() > 0 {
				return token.String()
			}
		} else if c == '{' || c == '}' {
			if token.Len() > 0 {
				s.Index--
				return token.String()
			}
			return string(c)
		} else if c == '/' && s.Index < length && buffer[s.Index] == '/' {
			if token.Len() > 0 {
				s.Index--
				return token.String()
			}
			inComment = true
		} else if c == '"' {
			if token.Len() > 0 {
				s.Index--
				return token.String()
			}
			inString = true
		} else {
			token.WriteByte(c)
		}
	}

	return ""
}

// Read reads the next token or returns an error at EOF.
// Empty quoted strings ("") are valid tokens and must not be treated as EOF.
func (s *TokenStream) Read() (string, error) {
	if s.Index >= len(s.Buffer) {
		return "", errors.New("End of stream reached prematurely")
	}
	indexBefore := s.Index
	value := s.ReadToken()
	if value == "" && s.Index == indexBefore {
		return "", errors.New("End of stream reached prematurely")
	}
	return value, nil
}

// Peek reads the next token without advancing.
func (s *TokenStream) Peek() (string, error) {
	index := s.Index
	value, err := s.Read()
	s.Index = index
	return value, err
}

// ReadInt reads the next token as int.
func (s *TokenStream) ReadInt() (int, error) {
	token, err := s.Read()
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(token)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// ReadFloat reads the next token as float64.
func (s *TokenStream) ReadFloat() (float64, error) {
	token, err := s.Read()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(token, 64)
}

// ReadVector reads { n0, n1, ..., nN } into view.
func (s *TokenStream) ReadVector(view []float32) ([]float32, error) {
	if _, err := s.Read(); err != nil { // {
		return nil, err
	}
	for i := range view {
		v, err := s.ReadFloat()
		if err != nil {
			return nil, err
		}
		view[i] = float32(v)
	}
	if _, err := s.Read(); err != nil { // }
		return nil, err
	}
	return view, nil
}

// ReadVectorU8 reads { ... } into a uint8 slice.
func (s *TokenStream) ReadVectorU8(view []uint8) ([]uint8, error) {
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	for i := range view {
		v, err := s.ReadFloat()
		if err != nil {
			return nil, err
		}
		view[i] = uint8(v)
	}
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	return view, nil
}

// ReadVectorsBlock reads a block of vectors into view with the given component size.
func (s *TokenStream) ReadVectorsBlock(view []float32, size int) ([]float32, error) {
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	for i := 0; i < len(view); i += size {
		if _, err := s.ReadVector(view[i : i+size]); err != nil {
			return nil, err
		}
	}
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	return view, nil
}

// ReadVectorsBlockU16 reads a block of vectors into uint16 view.
func (s *TokenStream) ReadVectorsBlockU16(view []uint16, size int) ([]uint16, error) {
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	for i := 0; i < len(view); i += size {
		if _, err := s.Read(); err != nil { // {
			return nil, err
		}
		for j := 0; j < size; j++ {
			v, err := s.ReadFloat()
			if err != nil {
				return nil, err
			}
			view[i+j] = uint16(v)
		}
		if _, err := s.Read(); err != nil { // }
			return nil, err
		}
	}
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	return view, nil
}

// ReadColor reads { R, G, B } sizzled to BGR in view.
func (s *TokenStream) ReadColor(view []float32) ([]float32, error) {
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	r, err := s.ReadFloat()
	if err != nil {
		return nil, err
	}
	g, err := s.ReadFloat()
	if err != nil {
		return nil, err
	}
	b, err := s.ReadFloat()
	if err != nil {
		return nil, err
	}
	view[2] = float32(r)
	view[1] = float32(g)
	view[0] = float32(b)
	if _, err := s.Read(); err != nil {
		return nil, err
	}
	return view, nil
}

// ReadBlock yields keys from a block until '}'.
func (s *TokenStream) ReadBlock() ([]string, error) {
	if _, err := s.Read(); err != nil { // {
		return nil, err
	}
	var keys []string
	for {
		token, err := s.Read()
		if err != nil {
			return nil, err
		}
		if token == "}" {
			break
		}
		keys = append(keys, token)
	}
	return keys, nil
}

// ReadBlockIter calls fn for each key in a block; fn should consume the value tokens.
func (s *TokenStream) ReadBlockIter(fn func(key string) error) error {
	if _, err := s.Read(); err != nil {
		return err
	}
	for {
		token, err := s.Read()
		if err != nil {
			return err
		}
		if token == "}" {
			return nil
		}
		if err := fn(token); err != nil {
			return err
		}
	}
}

func (s *TokenStream) WriteLine(line string) {
	s.Buffer += strings.Repeat(" ", s.Ident*s.IndentSpaces) + line + "\n"
}

func (s *TokenStream) WriteFlag(flag string) {
	s.WriteLine(flag + ",")
}

func (s *TokenStream) WriteFlagAttrib(name, flag string) {
	s.WriteLine(fmt.Sprintf("%s %s,", name, flag))
}

func (s *TokenStream) WriteNumberAttrib(name string, value float64) {
	s.WriteLine(fmt.Sprintf("%s %s,", name, s.FloatDecimals(value)))
}

func (s *TokenStream) WriteStringAttrib(name, value string) {
	s.WriteLine(fmt.Sprintf("%s \"%s\",", name, value))
}

func (s *TokenStream) WriteVectorAttrib(name string, value interface{}) {
	s.WriteLine(fmt.Sprintf("%s { %s },", name, s.floatArrayDecimals(value)))
}

func (s *TokenStream) WriteColor(name string, value []float32) {
	b := s.FloatDecimals(float64(value[0]))
	g := s.FloatDecimals(float64(value[1]))
	r := s.FloatDecimals(float64(value[2]))
	s.WriteLine(fmt.Sprintf("%s { %s, %s, %s },", name, r, g, b))
}

func (s *TokenStream) WriteVector(value interface{}) {
	s.WriteLine(fmt.Sprintf("{ %s },", s.floatArrayDecimals(value)))
}

func (s *TokenStream) WriteVectorArrayBlock(name string, view []float32, size int) {
	s.StartBlock(name, len(view)/size)
	for i := 0; i < len(view); i += size {
		s.WriteVector(view[i : i+size])
	}
	s.EndBlock()
}

func (s *TokenStream) StartBlock(name string, headers ...interface{}) {
	if len(headers) > 0 {
		parts := make([]string, len(headers))
		for i, h := range headers {
			parts[i] = fmt.Sprint(h)
		}
		name = name + " " + strings.Join(parts, " ")
	}
	s.WriteLine(name + " {")
	s.Ident++
}

func (s *TokenStream) StartObjectBlock(header, name string) {
	escaped := strings.ReplaceAll(name, `"`, `\"`)
	s.WriteLine(fmt.Sprintf("%s \"%s\" {", header, escaped))
	s.Ident++
}

func (s *TokenStream) EndBlock() {
	s.Ident--
	s.WriteLine("}")
}

func (s *TokenStream) EndBlockComma() {
	s.Ident--
	s.WriteLine("},")
}

func (s *TokenStream) Indent() {
	s.Ident++
}

func (s *TokenStream) Unindent() {
	s.Ident--
}

func (s *TokenStream) FloatDecimals(value float64) string {
	truncated := math.Trunc(value*s.Precision) / s.Precision
	return strconv.FormatFloat(truncated, 'f', -1, 64)
}

func (s *TokenStream) floatArrayDecimals(value interface{}) string {
	switch v := value.(type) {
	case []float32:
		parts := make([]string, len(v))
		for i, f := range v {
			parts[i] = s.FloatDecimals(float64(f))
		}
		return strings.Join(parts, ", ")
	case []uint8:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.Itoa(int(n))
		}
		return strings.Join(parts, ", ")
	case []uint16:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.Itoa(int(n))
		}
		return strings.Join(parts, ", ")
	case []uint32:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.Itoa(int(n))
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}
