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
	verify := flag.Bool("verify", false, "Verify that all the symbols are defined")
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
		fmt.Fprintf(os.Stderr, "ERROR: could not read file %s: %s\n", *filePath, err)
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
		for i := range names {
			fmt.Println(names[i])
		}
		return
	}

	expr, ok := grammar[*entry]
	if !ok {
		fmt.Printf("ERROR: Symbol %s is not defined. Pass -entry '!' to get the list of defined symbols.\n", *entry)
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
