package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"unicode"
	"sort"
	"time"
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
	TokenInvalid TokenKind = iota
	TokenSym
	TokenRuleSep
	TokenBar
	TokenStr
)

func TokenKindName(kind TokenKind) string {
	switch kind {
	case TokenInvalid:
		return "invalid token"
	case TokenSym:
		return "symbol"
	case TokenRuleSep:
		return "rule separator"
	case TokenBar:
		return "bar"
	case TokenStr:
		return "string literal"
	default:
		panic("unreachable")
	}
}

type Token struct {
	Kind TokenKind
	Text string
	Loc Loc
}

var EndToken = errors.New("end token")

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

func (lexer *Lexer) ChopToken() (token Token, err error) {
	lexer.Trim()

	token.Loc = lexer.Loc()

	if lexer.Col >= len(lexer.Content) {
		err = EndToken
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

		token.Kind = TokenSym
		token.Text = string(lexer.Content[begin:lexer.Col])
		lexer.Col += 1
		return
	}

	if lexer.Content[lexer.Col] == '"' {
		begin := lexer.Col + 1
		lexer.Col = begin

		var sb strings.Builder

		loop: for lexer.Col < len(lexer.Content) {
			switch lexer.Content[lexer.Col] {
			case '"':
				break loop
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
				case '"': sb.WriteRune('"')
				case 'n': sb.WriteRune('\n')
				default:
					err = &DiagErr{
						Loc: lexer.Loc(),
						Err: fmt.Errorf("Unknown escape sequence starting with %c", lexer.Content[lexer.Col]),
					}
					return
				}
			default:
				sb.WriteRune(lexer.Content[lexer.Col])
			}
			lexer.Col += 1
		}

		if lexer.Col >= len(lexer.Content) {
			err = &DiagErr{
				Loc: lexer.Loc(),
				Err: fmt.Errorf("Expected '\"' at the end of a string literal"),
			}
			return
		}

		token.Kind = TokenStr
		token.Text = sb.String()
		lexer.Col += 1
		return
	}

	RuleSep := []rune("::=")
	if lexer.Prefix(RuleSep) {
		token.Kind = TokenRuleSep
		token.Text = string(RuleSep)
		lexer.Col += len(RuleSep)
		return
	}

	Bar := []rune("|")
	if lexer.Prefix(Bar) {
		token.Kind = TokenBar
		token.Text = string(Bar)
		lexer.Col += len(Bar)
		return
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

type ExprKind int

const (
	ExprSym ExprKind = iota
	ExprStr
	ExprAlt
	ExprSeq
)

type Expr struct {
	Kind     ExprKind
	Loc      Loc
	Text     string
	Children []Expr
}

func (expr Expr) String() string {
	switch expr.Kind {
	case ExprSym:
		return fmt.Sprintf("<%s>", expr.Text)
	case ExprStr:
		// TODO: escape the string
		return fmt.Sprintf("\"%s\"", expr.Text)
	case ExprAlt:
		children := []string{}
		for i := range expr.Children {
			children = append(children, expr.Children[i].String())
		}
		return strings.Join(children, " | ")
	case ExprSeq:
		children := []string{}
		for i := range expr.Children {
			children = append(children, expr.Children[i].String())
		}
		return strings.Join(children, " ")
	default:
		panic("unreachable")
	}
}

func ExpectToken(lexer *Lexer, kind TokenKind) (token Token, err error) {
	token, err = lexer.Next()
	if err != nil {
		return
	}
	if token.Kind != kind {
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected %s but got %s", TokenKindName(kind), TokenKindName(token.Kind)),
		}
		return
	}
	return
}

func ParseAtomicExpr(lexer *Lexer) (expr Expr, err error) {
	var token Token
	token, err = lexer.Next()
	if err != nil {
		if err == EndToken {
			err = &DiagErr{
				Loc: token.Loc,
				Err: fmt.Errorf("Expected %s or %s, but got the end of the line", TokenKindName(TokenStr), TokenKindName(TokenSym)),
			}
		}
		return
	}
	switch token.Kind {
	case TokenSym:
		expr = Expr{
			Kind: ExprSym,
			Loc:  token.Loc,
			Text: token.Text,
		}
	case TokenStr:
		expr = Expr{
			Kind: ExprStr,
			Loc:  token.Loc,
			Text: token.Text,
		}
	default:
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected %s or %s, but got %s", TokenKindName(TokenStr), TokenKindName(TokenSym), TokenKindName(token.Kind)),
		}
	}
	return
}

func ParseSeqExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParseAtomicExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil || (token.Kind != TokenSym && token.Kind != TokenStr) {
		if err == EndToken {
			err = nil
		}
		return
	}

	expr = Expr{
		Loc:      expr.Loc,
		Kind:     ExprSeq,
		Children: []Expr{expr},
	}

	for err == nil && (token.Kind == TokenSym || token.Kind == TokenStr) {
		var child Expr
		child, err = ParseAtomicExpr(lexer)
		if err != nil {
			return
		}
		expr.Children = append(expr.Children, child)
		token, err = lexer.Peek()
	}

	if err == EndToken {
		err = nil
	}

	return
}

func ParseAltExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParseSeqExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil || token.Kind != TokenBar {
		if err == EndToken {
			err = nil
		}
		return
	}

	expr = Expr{
		Loc:      expr.Loc,
		Kind:     ExprAlt,
		Children: []Expr{expr},
	}

	for err == nil && token.Kind == TokenBar {
		token, err = ExpectToken(lexer, TokenBar)
		if err != nil {
			return
		}
		var child Expr
		child, err = ParseSeqExpr(lexer)
		if err != nil {
			return
		}
		expr.Children = append(expr.Children, child)
		token, err = lexer.Peek()
	}

	if err == EndToken {
		err = nil
	}
	return
}

func ParseExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParseAltExpr(lexer)
	return
}

type Rule struct {
	Head Token
	Body Expr
}

func ParseRule(lexer *Lexer) (rule Rule, err error) {
	rule.Head, err = ExpectToken(lexer, TokenSym)
	if err != nil {
		return
	}
	_, err = ExpectToken(lexer, TokenRuleSep)
	if err != nil {
		return
	}
	rule.Body, err = ParseExpr(lexer)
	return
}

// TODO: limit the amount of loops
func GenerateRandomMessage(grammar map[string]Rule, expr Expr) (message string, err error) {
	switch expr.Kind {
	case ExprStr:
		message = expr.Text
	case ExprSym:
		nextExpr, ok := grammar[expr.Text]
		if !ok {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Symbol <%s> is not defined", expr.Text),
			}
			return
		}
		message, err = GenerateRandomMessage(grammar, nextExpr.Body)
	case ExprSeq:
		var sb strings.Builder
		for i := range expr.Children {
			var childMessage string
			childMessage, err = GenerateRandomMessage(grammar, expr.Children[i])
			if err != nil {
				return
			}
			sb.WriteString(childMessage)
		}
		message = sb.String()
	case ExprAlt:
		i := rand.Int31n(int32(len(expr.Children)))
		message, err = GenerateRandomMessage(grammar, expr.Children[i])
	}
	return
}

func main() {
	rand.Seed(time.Now().UnixNano())
	filePath := flag.String("file", "", "Path to the BNF file")
	entry := flag.String("entry", "", "The symbol name to start generating from. Passing '!' as the symbol name lists all of the available symbols in the -file.")
	count := flag.Int("count", 1, "How many messages to generate")
	flag.Parse()
	if len(*filePath) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -file is not provided\n")
		flag.Usage()
		os.Exit(1)
	}
	if len(*entry) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: -symbol is not provided\n")
		flag.Usage()
		os.Exit(1)
	}
	content, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: could not read file %s: %s\n", filePath, err)
		os.Exit(1)
	}
	grammar := map[string]Rule{}
	for row, line := range strings.Split(string(content), "\n") {
		if len(strings.TrimSpace(line)) > 0 {
			lexer := NewLexer(line, *filePath, row)
			rule, err := ParseRule(&lexer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				os.Exit(1)
			}
			// TODO: check for redefinition
			grammar[rule.Head.Text] = rule
		}
	}

	if *entry == "!" {
		names := []string{}
		for name := range grammar {
			names = append(names, name)
		}
		sort.Strings(names)
		for i := range names {
			fmt.Println(names[i])
		}
		return
	}

	expr, ok := grammar[*entry]
	if !ok {
		fmt.Printf("ERROR: Symbol %s is not defined\n", *entry)
		os.Exit(1)
	}

	for i := 0; i < *count; i += 1 {
		message, err := GenerateRandomMessage(grammar, expr.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		fmt.Println(message)
	}
}
