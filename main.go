package main

import (
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

type ExprKind int

const (
	ExprSymbol ExprKind = iota
	ExprString
	ExprAlternation
	ExprConcat
	ExprRepetition
	ExprRange
)

type Expr struct {
	Kind     ExprKind
	Loc      Loc
	Text     []rune
	Children []Expr
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

func ParsePrimaryExpr(lexer *Lexer) (expr Expr, err error) {
	var token Token
	token, err = lexer.Next()
	if err != nil {
		return
	}
	if token.Kind == TokenEOL {
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected start of an expression, but got %s", TokenKindName(token.Kind)),
		}
		return
	}
	switch token.Kind {
	case TokenCurlyOpen:
		expr, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenCurlyClose)
		if err != nil {
			return
		}
		expr = Expr{
			Kind: ExprRepetition,
			Loc: token.Loc,
			Children: []Expr{
				expr,
			},
		}
	case TokenBracketOpen:
		expr, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenBracketClose)
		if err != nil {
			return
		}
		expr = Expr{
			Kind: ExprAlternation,
			Loc: token.Loc,
			Children: []Expr{
				expr,
				Expr{
					Kind: ExprString,
					Loc: token.Loc,
				},
			},
		}
	case TokenSymbol:
		expr = Expr{
			Kind: ExprSymbol,
			Loc:  token.Loc,
			Text: token.Text,
		}
	case TokenString:
		expr = Expr{
			Kind: ExprString,
			Loc:  token.Loc,
			Text: token.Text,
		}
		var ellipsis Token
		ellipsis, err = lexer.Peek()
		if err != nil {
			return
		}
		if ellipsis.Kind == TokenEllipsis {
			if len(expr.Text) != 1 {
				err = &DiagErr{
					Loc: expr.Loc,
					Err: fmt.Errorf("The lower boundary of the range is expected to be 1 symbol string. Got %d instead.", len(expr.Text)),
				}
				return
			}

			lexer.PeekFull = false
			var upper Token

			upper, err = ExpectToken(lexer, TokenString)
			if err != nil {
				return
			}

			if len(upper.Text) != 1 {
				err = &DiagErr{
					Loc: upper.Loc,
					Err: fmt.Errorf("The upper boundary of the range is expected to be 1 symbol string. Got %d instead.", len(upper.Text)),
				}
				return
			}

			expr = Expr{
				Kind: ExprRange,
				Loc: ellipsis.Loc,
				Text: []rune{
					expr.Text[0],
					upper.Text[0],
				},
			}
		}
	default:
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected start of an expression, but got %s", TokenKindName(token.Kind)),
		}
	}
	return
}

func ParseConcatExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParsePrimaryExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil {
		return
	}
	if token.Kind != TokenSymbol && token.Kind != TokenString && token.Kind != TokenBracketOpen && token.Kind != TokenCurlyOpen {
		return
	}

	expr = Expr{
		Loc:      expr.Loc,
		Kind:     ExprConcat,
		Children: []Expr{expr},
	}

	for err == nil && (token.Kind == TokenSymbol || token.Kind == TokenString || token.Kind == TokenBracketOpen || token.Kind == TokenCurlyOpen) {
		var child Expr
		child, err = ParsePrimaryExpr(lexer)
		if err != nil {
			return
		}
		expr.Children = append(expr.Children, child)
		token, err = lexer.Peek()
	}

	return
}

func ParseAltExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParseConcatExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil {
		return
	}
	if token.Kind != TokenAlternation {
		return
	}

	expr = Expr{
		Loc:      expr.Loc,
		Kind:     ExprAlternation,
		Children: []Expr{expr},
	}

	for err == nil && token.Kind == TokenAlternation {
		token, err = ExpectToken(lexer, TokenAlternation)
		if err != nil {
			return
		}
		var child Expr
		child, err = ParseConcatExpr(lexer)
		if err != nil {
			return
		}
		expr.Children = append(expr.Children, child)
		token, err = lexer.Peek()
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
	rule.Head, err = ExpectToken(lexer, TokenSymbol)
	if err != nil {
		return
	}
	_, err = ExpectToken(lexer, TokenDefinition)
	if err != nil {
		return
	}
	rule.Body, err = ParseExpr(lexer)
	return
}

// TODO: limit the amount of loops
func GenerateRandomMessage(grammar map[string]Rule, expr Expr) (message []rune, err error) {
	switch expr.Kind {
	case ExprString:
		message = expr.Text
	case ExprSymbol:
		symbol := string(expr.Text)
		nextExpr, ok := grammar[symbol]
		if !ok {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Symbol <%s> is not defined", symbol),
			}
			return
		}
		message, err = GenerateRandomMessage(grammar, nextExpr.Body)
	case ExprConcat:
		for i := range expr.Children {
			var childMessage []rune
			childMessage, err = GenerateRandomMessage(grammar, expr.Children[i])
			if err != nil {
				return
			}
			message = append(message, childMessage...)
		}
	case ExprAlternation:
		i := rand.Int31n(int32(len(expr.Children)))
		message, err = GenerateRandomMessage(grammar, expr.Children[i])
	case ExprRepetition:
		// TODO: customizable MaxRepetition
		MaxRepetition := 20
		n := int(rand.Int31n(int32(MaxRepetition)))
		for i := 0; i < n; i += 1 {
			for j := range expr.Children {
				var childMessage []rune
				childMessage, err = GenerateRandomMessage(grammar, expr.Children[j])
				if err != nil {
					return
				}
				message = append(message, childMessage...)
			}
		}
	case ExprRange:
		if len(expr.Text) != 2 {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Unexpected arity of range. Expected 2 but got %d.", len(expr.Text)),
			}
			return
		}

		if expr.Text[0] > expr.Text[1] {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Upper bound of the range is lower than the lower one."),
			}
			return
		}

		message = append(message, expr.Text[0] + rand.Int31n(expr.Text[1] - expr.Text[0] + 1))
	default:
		panic("unreachable")
	}
	return
}

func VerifyThatAllSymbolsDefinedInExpr(grammar map[string]Rule, expr Expr) (ok bool) {
	ok = true
	switch expr.Kind {
	case ExprSymbol:
		symbol := string(expr.Text)
		if _, symbolExists := grammar[symbol]; !symbolExists {
			ok = false
			fmt.Fprintf(os.Stderr, "%s: ERROR: Symbol %s is not defined\n", expr.Loc, symbol)
		}
		return

	case ExprAlternation:
		fallthrough
	case ExprConcat:
		fallthrough
	case ExprRepetition:
		for _, child := range expr.Children {
			if !VerifyThatAllSymbolsDefinedInExpr(grammar, child) {
				ok = false
			}
		}
		return

	case ExprString:
		fallthrough
	case ExprRange:
		return

	default: panic("unreachable")
	}
}

func VerifyThatAllSymbolsDefined(grammar map[string]Rule) (ok bool) {
	ok = true
	for _, expr := range grammar {
		if !VerifyThatAllSymbolsDefinedInExpr(grammar, expr.Body) {
			ok = false
		}
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
		fmt.Fprintf(os.Stderr, "ERROR: -entry is not provided\n")
		flag.Usage()
		os.Exit(1)
	}
	content, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: could not read file %s: %s\n", filePath, err)
		os.Exit(1)
	}
	grammar := map[string]Rule{}
	parsingError := false
	for row, line := range strings.Split(string(content), "\n") {
		lexer := NewLexer(line, *filePath, row)

		token, err := lexer.Peek()
		if err == nil && token.Kind == TokenEOL {
			continue
		}

		newRule, err := ParseRule(&lexer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			parsingError = true
			continue
		}

		_, err = ExpectToken(&lexer, TokenEOL)
		if err != nil {
			fmt.Fprintf(os. Stderr, "%s\n", err)
			parsingError = true
			continue
		}

		symbol := string(newRule.Head.Text)
		existingRule, ok := grammar[symbol]
		if ok {
			fmt.Fprintf(os.Stderr, "%s: ERROR: redefinition of the rule %s\n", newRule.Head.Loc, symbol)
			fmt.Fprintf(os.Stderr, "%s: NOTE: the first definition is located here\n", existingRule.Head.Loc)
			parsingError = true
			continue
		}

		grammar[symbol] = newRule
	}

	if parsingError {
		os.Exit(1)
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

	ok = VerifyThatAllSymbolsDefined(grammar)
	if !ok {
		os.Exit(1)
	}

	for i := 0; i < *count; i += 1 {
		message, err := GenerateRandomMessage(grammar, expr.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		fmt.Println(string(message))
	}
}
