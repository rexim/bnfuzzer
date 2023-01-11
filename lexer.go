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
	TokenEllipsis
)

func TokenKindName(kind TokenKind) string {
	switch kind {
	case TokenEOL:
		return "end of line"
	case TokenSymbol:
		return "symbol"
	case TokenDefinition:
		return "definition symbol"
	case TokenAlternation:
		return "alternation symbol"
	case TokenString:
		return "string literal"
	case TokenBracketOpen:
		return "open bracket"
	case TokenBracketClose:
		return "close bracket"
	case TokenCurlyOpen:
		return "open curly"
	case TokenCurlyClose:
		return "close curly"
	case TokenEllipsis:
		return "ellipsis"
	default:
		panic("unreachable")
	}
}

type Token struct {
	Kind TokenKind
	Text []rune
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

func (lexer *Lexer) ChopStrLit() (lit []rune, err error) {
	if lexer.Col >= len(lexer.Content) {
		return
	}

	quote := lexer.Content[lexer.Col]
	lexer.Col += 1
	begin := lexer.Col

	loop: for lexer.Col < len(lexer.Content) {
		switch lexer.Content[lexer.Col] {
		case '\\':
			if lexer.Col + 1 >= len(lexer.Content) {
				err = &DiagErr{
					Loc: lexer.Loc(),
					Err: fmt.Errorf("Unfinished escape sequence"),
				}
				return
			}
			lexer.Col += 1
			switch lexer.Content[lexer.Col] {
			case 'n': lit = append(lit, '\n')
			case 'r': lit = append(lit, '\r')
			case '\\': lit = append(lit, '\\')
			default:
				if lexer.Content[lexer.Col] == quote {
					lit = append(lit, quote)
				} else {
					err = &DiagErr{
						Loc: lexer.Loc(),
						Err: fmt.Errorf("Unknown escape sequence starting with %c", lexer.Content[lexer.Col]),
					}
					return
				}
			}
		default:
			if lexer.Content[lexer.Col] == quote {
				break loop
			}
			lit = append(lit, lexer.Content[lexer.Col])
		}
		lexer.Col += 1
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

func (lexer *Lexer) ChopToken() (token Token, err error) {
	lexer.Trim()

	if lexer.Prefix([]rune("//")) {
		lexer.Col = len(lexer.Content)
	}

	token.Loc = lexer.Loc()

	if lexer.Col >= len(lexer.Content) {
		return
	}

	if lexer.Content[lexer.Col] == '<' {
		begin := lexer.Col + 1
		lexer.Col = begin
		for lexer.Col < len(lexer.Content) && lexer.Content[lexer.Col] != '>' {
			ch := lexer.Content[lexer.Col]
			if !unicode.IsLetter(ch) && !unicode.IsNumber(ch) && ch != '-' && ch != '_' {
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

	LiteralTokens := map[string]TokenKind {
		"::=": TokenDefinition,
		"=": TokenDefinition,
		"|": TokenAlternation,
		"[": TokenBracketOpen,
		"]": TokenBracketClose,
		"{": TokenCurlyOpen,
		"}": TokenCurlyClose,
		"...": TokenEllipsis,
	}

	for name, kind := range LiteralTokens {
		runeName := []rune(name)
		if lexer.Prefix(runeName) {
			token.Kind = kind
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
