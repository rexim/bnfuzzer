package main

import "fmt"

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
			Err: fmt.Errorf("Expected %s but got %s", TokenKindName[kind], TokenKindName[token.Kind]),
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
	switch token.Kind {
	case TokenParenOpen:
		expr, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenParenClose)
		if err != nil {
			return
		}
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

			var dash Token
			dash, err = lexer.Peek()
			if err != nil {
				return
			}
			if dash.Kind == TokenDash {
				lexer.PeekFull = false
				var except Token
				except, err = ExpectToken(lexer, TokenString)
				if err != nil {
					return
				}

				expr.Text = append(expr.Text, except.Text...)
			}
		}

	case TokenNumber:
		panic("TODO: variable repetition without lower bound")

	case TokenNumber:
		var asterisk Token
		asterisk, err = lexer.Peek()
		if err != nil {
			return
		}
		if asterisk.Kind != TokenAsterisk {
			panic("TODO: specific repetition")
		}

		panic("TODO: variable repetition with lower bound")
	default:
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected start of an expression, but got %s", TokenKindName[token.Kind]),
		}
	}
	return
}

func IsPrimaryStart(kind TokenKind) bool {
	return kind == TokenSymbol ||
		kind == TokenString ||
		kind == TokenBracketOpen ||
		kind == TokenCurlyOpen ||
		kind == TokenParenOpen ||
		kind == TokenNumber
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
	if !IsPrimaryStart(token.Kind) {
		return
	}

	expr = Expr{
		Loc:      expr.Loc,
		Kind:     ExprConcat,
		Children: []Expr{expr},
	}

	for err == nil && IsPrimaryStart(token.Kind) {
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
