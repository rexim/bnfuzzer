package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sort"
	"time"
)

// TODO: limit the amount of loops
func GenerateRandomMessage(grammar map[string]Rule, expr Expr) (message []rune, err error) {
	switch expr := expr.(type) {
	case ExprString:
		message = expr.Text
	case ExprSymbol:
		nextExpr, ok := grammar[expr.Name]
		if !ok {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Symbol <%s> is not defined", expr.Name),
			}
			return
		}
		message, err = GenerateRandomMessage(grammar, nextExpr.Body)
	case ExprConcat:
		for i := range expr.Elements {
			var element []rune
			element, err = GenerateRandomMessage(grammar, expr.Elements[i])
			if err != nil {
				return
			}
			message = append(message, element...)
		}
	case ExprAlternation:
		i := rand.Int31n(int32(len(expr.Variants)))
		message, err = GenerateRandomMessage(grammar, expr.Variants[i])
	case ExprRepetition:
		if expr.Lower > expr.Upper {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Upper bound of the repetition is lower than the lower one."),
			}
			return
		}
		n := int(int32(expr.Lower) + rand.Int31n(int32(expr.Upper - expr.Lower + 1)))
		for i := 0; i < n; i += 1 {
			var childMessage []rune
			childMessage, err = GenerateRandomMessage(grammar, expr.Body)
			if err != nil {
				return
			}
			message = append(message, childMessage...)
		}
	case ExprRange:
		if expr.Lower > expr.Upper {
			err = &DiagErr{
				Loc: expr.Loc,
				Err: fmt.Errorf("Upper bound of the range is lower than the lower one."),
			}
			return
		}

		message = append(message, expr.Lower + rand.Int31n(expr.Upper - expr.Lower + 1))
	default:
		panic("unreachable")
	}
	return
}

func VerifyThatAllSymbolsDefinedInExpr(grammar map[string]Rule, expr Expr) (ok bool) {
	ok = true
	switch expr := expr.(type) {
	case ExprSymbol:
		if _, exists := grammar[expr.Name]; !exists {
			ok = false
			fmt.Fprintf(os.Stderr, "%s: ERROR: Symbol %s is not defined\n", expr.Loc, expr.Name)
		}
		return

	case ExprAlternation:
		for i := range expr.Variants {
			if !VerifyThatAllSymbolsDefinedInExpr(grammar, expr.Variants[i]) {
				ok = false
			}
		}
		return

	case ExprConcat:
		for i := range expr.Elements {
			if !VerifyThatAllSymbolsDefinedInExpr(grammar, expr.Elements[i]) {
				ok = false
			}
		}
		return

	case ExprRepetition:
		if !VerifyThatAllSymbolsDefinedInExpr(grammar, expr.Body) {
			ok = false
		}
		return

	case ExprString:
		return

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

func WalkSymbolsInExpr(grammar map[string]Rule, expr Expr, visited map[string]bool) (err error) {
	switch expr := expr.(type) {
	case ExprSymbol:
		if !visited[expr.Name] {
			visited[expr.Name] = true
			rule, exists := grammar[expr.Name]
			if !exists {
				err = &DiagErr{
					Loc: expr.Loc,
					Err: fmt.Errorf("Symbol <%s> is not defined", expr.Name),
				}
				return
			}
			err = WalkSymbolsInExpr(grammar, rule.Body, visited)
			if err != nil {
				return
			}
		}
		return
	case ExprString:
		return
	case ExprAlternation:
		for i := range expr.Variants {
			err = WalkSymbolsInExpr(grammar, expr.Variants[i], visited)
			if err != nil {
				return
			}
		}
		return
	case ExprConcat:
		for i := range expr.Elements {
			err = WalkSymbolsInExpr(grammar, expr.Elements[i], visited)
			if err != nil {
				return
			}
		}
		return
	case ExprRepetition:
		return WalkSymbolsInExpr(grammar, expr.Body, visited)
	case ExprRange:
		return
	}
	panic(fmt.Sprintf("unreachable: %T", expr))
}

type Rule struct {
	Head Token
	Body Expr
}

func (rule Rule) String() string {
	sep := ""
	for i := range LiteralTokens {
		if LiteralTokens[i].Kind == TokenDefinition {
			sep = LiteralTokens[i].Text
			break
		}
	}
	if len(sep) == 0 {
		// This should be possible to check at compile time in 2023
		panic("Not a single TokenAlternation exists to render ExprAlternation")
	}

	sb := strings.Builder{}
	sb.WriteString(string(rule.Head.Text))
	sb.WriteString(" "+sep+" ")
	sb.WriteString(rule.Body.String())
	return sb.String()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	filePath := flag.String("file", "", "Path to the BNF file")
	entry := flag.String("entry", "", "The symbol name to start generating from. Passing '!' as the symbol name lists all of the available symbols in the -file.")
	count := flag.Int("count", 1, "How many messages to generate")
	verify := flag.Bool("verify", false, "Verify that all the symbols are defined")
	unused := flag.Bool("unused", false, "Verify that all the symbols are used")
	dump := flag.Bool("dump", false, "Dump the text representation of -entry symbol")
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
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
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

		var head Token
		head, err = ExpectToken(&lexer, TokenSymbol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			parsingError = true
			continue
		}

		var def Token
		def, err = lexer.Next()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			parsingError = true
			continue
		}

		symbol := string(head.Text)
		existingRule, ruleExists := grammar[symbol]

		switch def.Kind {
		case TokenDefinition:
			if ruleExists {
				fmt.Fprintf(os.Stderr, "%s: ERROR: redefinition of the rule %s\n", head.Loc, symbol)
				fmt.Fprintf(os.Stderr, "%s: NOTE: the first definition is located here\n", existingRule.Head.Loc)
				parsingError = true
				continue
			}

			var body Expr
			body, err = ParseExpr(&lexer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				parsingError = true
				continue
			}

			grammar[symbol] = Rule{
				Head: head,
				Body: body,
			}

		case TokenIncAlternative:
			if !ruleExists {
				fmt.Fprintf(os.Stderr, "%s: ERROR: can't apply incremental alternative to a non-existing rule %s. You need to define it first.\n", head.Loc, symbol)
				parsingError = true
				continue
			}

			var body Expr
			body, err = ParseExpr(&lexer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				parsingError = true
				continue
			}

			switch existingBody := existingRule.Body.(type) {
			case ExprAlternation:
				existingBody.Variants = append(existingBody.Variants, body)
				existingRule.Body = existingBody
			default:
				existingRule.Body = ExprAlternation{
					Loc: existingBody.GetLoc(),
					Variants: []Expr{
						existingBody,
						body,
					},
				}
			}

			grammar[symbol] = existingRule
		default:
			fmt.Fprintf(os.Stderr, "%s\n", &DiagErr{
				Loc: def.Loc,
				Err: fmt.Errorf("Expected %s or %s but got %s",
					TokenKindName[TokenDefinition], TokenKindName[TokenIncAlternative],
					TokenKindName[def.Kind]),
			})
			parsingError = true
			continue
		}

		_, err = ExpectToken(&lexer, TokenEOL)
		if err != nil {
			fmt.Fprintf(os. Stderr, "%s\n", err)
			parsingError = true
			continue
		}
	}

	if parsingError {
		os.Exit(1)
	}

	if *verify {
		ok := VerifyThatAllSymbolsDefined(grammar)
		if !ok {
			os.Exit(1)
		}
	}

	if *entry == "!" {
		names := []string{}
		for name := range grammar {
			names = append(names, name)
		}
		sort.Strings(names)

		if *dump {
			for i := range names {
				rule := grammar[names[i]]
				fmt.Printf("%s: %s\n", rule.Head.Loc, rule.String())
			}
			return
		}

		for i := range names {
			fmt.Println(names[i])
		}
		return
	}

	rule, ok := grammar[*entry]
	if !ok {
		fmt.Printf("ERROR: Symbol %s is not defined. Pass -entry '!' to get the list of defined symbols.\n", *entry)
		os.Exit(1)
	}

	if *unused {
		visited := map[string]bool{}
		visited[*entry] = true
		err = WalkSymbolsInExpr(grammar, rule.Body, visited)

		ok := true
		for name := range grammar {
			if !visited[name] {
				fmt.Fprintf(os.Stderr, "%s: %s is unused\n", grammar[name].Head.Loc, name)
				ok = false
			}
		}
		if !ok {
			os.Exit(1)
		}
	}

	if *dump {
		fmt.Printf("%s: %s\n", rule.Head.Loc, rule.String())
		return
	}

	for i := 0; i < *count; i += 1 {
		message, err := GenerateRandomMessage(grammar, rule.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		fmt.Print(string(message))
	}
}
