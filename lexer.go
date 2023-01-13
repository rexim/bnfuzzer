package main

import (
	"fmt"
	"unicode"
)

type Loc struct {
	FilePath string
	Row int
	Col int
}

type DiagErr struct {
	Loc Loc
	Err error
}

func (err *DiagErr) Error() string {
	return fmt.Sprintf("%s: ERROR: %s", err.Loc, err.Err)
}

func (loc Loc) String() string {
	return fmt.Sprintf("%s:%d:%d", loc.FilePath, loc.Row + 1, loc.Col + 1)
}

type Lexer struct {
	Content  []rune
	FilePath string
	Row int
	Col int
	PeekBuf  Token
	PeekFull bool
}

func NewLexer(content string, filePath string, row int) Lexer {
	return Lexer{
		Content: []rune(content),
		FilePath: filePath,
		Row: row,
	}
}

type TokenKind int

const (
	TokenEOL TokenKind = iota
	TokenSymbol
	TokenDefinition
	TokenAlternation
	TokenString
	TokenBracketOpen
	TokenBracketClose
	TokenCurlyOpen
	TokenCurlyClose
	TokenParenOpen
	TokenParenClose
	TokenEllipsis
	TokenNumber
	TokenAsterisk
	TokenIncAlternative
	TokenValueRange
)

var TokenKindName = map[TokenKind]string{
	TokenEOL: "end of line",
	TokenSymbol: "symbol",
	TokenDefinition: "definition symbol",
	TokenAlternation: "alternation symbol",
	TokenString: "string literal",
	TokenBracketOpen: "open bracket",
	TokenBracketClose: "close bracket",
	TokenCurlyOpen: "open curly",
	TokenCurlyClose: "close curly",
	TokenParenOpen: "open paren",
	TokenParenClose: "close paren",
	TokenEllipsis: "ellipsis",
	TokenNumber: "number",
	TokenAsterisk: "asterisk",
	TokenIncAlternative: "incremental alternative",
	TokenValueRange: "value range",
}

type LiteralToken struct {
	Text string
	Kind TokenKind
}

var LiteralTokens = []LiteralToken{
	{ Text: "::=", Kind: TokenDefinition },
	{ Text: "=/", Kind: TokenIncAlternative },
	{ Text: "=", Kind: TokenDefinition },
	{ Text: "|", Kind: TokenAlternation },
	{ Text: "/", Kind: TokenAlternation },
	{ Text: "[", Kind: TokenBracketOpen },
	{ Text: "]", Kind: TokenBracketClose },
	{ Text: "{", Kind: TokenCurlyOpen },
	{ Text: "}", Kind: TokenCurlyClose },
	{ Text: "(", Kind: TokenParenOpen },
	{ Text: ")", Kind: TokenParenClose },
	{ Text: "...", Kind: TokenEllipsis },
	{ Text: "*", Kind: TokenAsterisk },
}

type Token struct {
	Kind TokenKind
	Text []rune
	Number uint
	Loc Loc
}

func (lexer *Lexer) Trim() {
	for lexer.Col < len(lexer.Content) && unicode.IsSpace(lexer.Content[lexer.Col]) {
		lexer.Col += 1
	}
}

func (lexer *Lexer) Index(x rune) int {
	for i := lexer.Col; i < len(lexer.Content); i += 1 {
		if lexer.Content[i] == x {
			return i
		}
	}
	return -1
}

func (lexer *Lexer) Prefix(prefix []rune) bool {
	for i := range prefix {
		if lexer.Col+i >= len(lexer.Content) {
			return false
		}
		if lexer.Content[lexer.Col+i] != prefix[i] {
			return false
		}
	}
	return true
}

func (lexer *Lexer) Loc() Loc {
	return Loc{
		FilePath: lexer.FilePath,
		Row: lexer.Row,
		Col: lexer.Col,
	}
}

func (lexer *Lexer) ChopHexByteValue() (result rune, err error) {
	for i := 0; i < 2; i += 1 {
		if lexer.Col >= len(lexer.Content) {
			err = &DiagErr{
				Loc: lexer.Loc(),
				Err: fmt.Errorf("Unfinished hexadecimal value of a byte. Expected 2 hex digits, but got %d.", i),
			}
			return
		}
		x := lexer.Content[lexer.Col]
		if '0' <= x && x <= '9' {
			result = result*0x10 + x - '0'
		} else if 'a' <= x && x <= 'f' {
			result = result*0x10 + x - 'a' + 10
		} else if 'A' <= x && x <= 'F' {
			result = result*0x10 + x - 'A' + 10
		} else {
			err = &DiagErr{
				Loc: lexer.Loc(),
				Err: fmt.Errorf("Expected hex digit, but got `%c`", x),
			}
			return
		}
		lexer.Col += 1
	}
	return
}

func (lexer *Lexer) ChopStrLit() (lit []rune, err error) {
	if lexer.Col >= len(lexer.Content) {
		return
	}

	quote := lexer.Content[lexer.Col]
	lexer.Col += 1
	begin := lexer.Col

	loop: for lexer.Col < len(lexer.Content) {
		if lexer.Content[lexer.Col] == '\\' {
			lexer.Col += 1
			if lexer.Col >= len(lexer.Content) {
				err = &DiagErr{
					Loc: lexer.Loc(),
					Err: fmt.Errorf("Unfinished escape sequence"),
				}
				return
			}

			switch lexer.Content[lexer.Col] {
			case '0':
				lit = append(lit, 0)
				lexer.Col += 1
			case 'n':
				lit = append(lit, '\n')
				lexer.Col += 1
			case 'r':
				lit = append(lit, '\r')
				lexer.Col += 1
			case '\\':
				lit = append(lit, '\\')
				lexer.Col += 1
			case 'x':
				lexer.Col += 1
				var value rune
				value, err = lexer.ChopHexByteValue()
				if err != nil {
					return
				}
				lit = append(lit, value)
			default:
				if lexer.Content[lexer.Col] == quote {
					lit = append(lit, quote)
					lexer.Col += 1
				} else {
					err = &DiagErr{
						Loc: lexer.Loc(),
						Err: fmt.Errorf("Unknown escape sequence starting with %c", lexer.Content[lexer.Col]),
					}
					return
				}
			}
		} else {
			if lexer.Content[lexer.Col] == quote {
				break loop
			}
			lit = append(lit, lexer.Content[lexer.Col])
			lexer.Col += 1
		}
	}

	if lexer.Col >= len(lexer.Content) || lexer.Content[lexer.Col] != quote {
		err = &DiagErr{
			Loc: Loc{
				FilePath: lexer.FilePath,
				Row: lexer.Row,
				Col: begin,
			},
			Err: fmt.Errorf("Expected '%c' at the end of this string literal", quote),
		}
		return
	}
	lexer.Col += 1

	return
}

func IsSymbolStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '-' || ch == '_'
}

func IsSymbol(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsNumber(ch) || ch == '-' || ch == '_'
}

func (lexer *Lexer) ChopToken() (token Token, err error) {
	lexer.Trim()

	if lexer.Prefix([]rune("//")) || lexer.Prefix([]rune(";")) {
		lexer.Col = len(lexer.Content)
	}

	token.Loc = lexer.Loc()

	if lexer.Col >= len(lexer.Content) {
		return
	}

	if unicode.IsNumber(lexer.Content[lexer.Col]) {
		begin := lexer.Col
		token.Number = 0
		for lexer.Col < len(lexer.Content) && unicode.IsNumber(lexer.Content[lexer.Col]) {
			token.Number *= 10
			token.Number += uint(lexer.Content[lexer.Col] - '0')
			lexer.Col += 1
		}
		token.Kind = TokenNumber
		token.Text = lexer.Content[begin:lexer.Col]
		return
	}

	if IsSymbolStart(lexer.Content[lexer.Col]) {
		begin := lexer.Col

		for lexer.Col < len(lexer.Content) && IsSymbol(lexer.Content[lexer.Col]) {
			lexer.Col += 1
		}

		token.Kind = TokenSymbol
		token.Text = lexer.Content[begin:lexer.Col]
		return
	}

	if lexer.Content[lexer.Col] == '<' {
		begin := lexer.Col + 1
		lexer.Col = begin
		for lexer.Col < len(lexer.Content) && lexer.Content[lexer.Col] != '>' {
			ch := lexer.Content[lexer.Col]
			if !IsSymbol(ch) {
				err = &DiagErr{
					Loc: lexer.Loc(),
					Err: fmt.Errorf("Unexpected character in symbol name %c", ch),
				}
				return
			}
			lexer.Col += 1
		}
		if lexer.Col >= len(lexer.Content) {
			err = &DiagErr{
				Loc: lexer.Loc(),
				Err: fmt.Errorf("Expected '>' at the end of the symbol name"),
			}
			return
		}

		token.Kind = TokenSymbol
		token.Text = lexer.Content[begin:lexer.Col]
		lexer.Col += 1
		return
	}

	if lexer.Content[lexer.Col] == '"' || lexer.Content[lexer.Col] == '\'' {
		var lit []rune
		lit, err = lexer.ChopStrLit()
		if err != nil {
			return
		}
		token.Kind = TokenString
		token.Text = lit
		return
	}
	if lexer.Prefix([]rune("%x")) {
		lexer.Col += 2

		var lower, upper rune

		lower, err = lexer.ChopHexByteValue()
		if err != nil {
			return
		}

		if !lexer.Prefix([]rune("-")) {
			err = &DiagErr{
				Loc: lexer.Loc(),
				Err: fmt.Errorf("Expected dash between lower and upper bounds of value range token"),
			}
			return
		}
		lexer.Col += 1

		upper, err = lexer.ChopHexByteValue()
		if err != nil {
			return
		}

		token.Kind = TokenValueRange
		token.Text = []rune{lower, upper}
		return
	}

	for i := range LiteralTokens {
		runeName := []rune(LiteralTokens[i].Text)
		if lexer.Prefix(runeName) {
			token.Kind = LiteralTokens[i].Kind
			token.Text = runeName
			lexer.Col += len(runeName)
			return
		}
	}

	err = &DiagErr{
		Loc: lexer.Loc(),
		Err: fmt.Errorf("Invalid token"),
	}
	return
}

func (lexer *Lexer) Peek() (token Token, err error) {
	if !lexer.PeekFull {
		token, err = lexer.ChopToken()
		if err != nil {
			return
		}
		lexer.PeekFull = true
		lexer.PeekBuf = token
	} else {
		token = lexer.PeekBuf
	}
	return
}

func (lexer *Lexer) Next() (token Token, err error) {
	if lexer.PeekFull {
		token = lexer.PeekBuf
		lexer.PeekFull = false
		return
	}

	token, err = lexer.ChopToken()
	return
}
