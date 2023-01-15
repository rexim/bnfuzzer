# BNF Fuzzer

Generate random messages based on their [BNF](https://en.wikipedia.org/wiki/Backus%E2%80%93Naur_form) definition.

## Quickl Start

Generate 10 random postal addresses:

```console
$ go build .
$ ./bnfuzzer -file ./examples/postal.bnf -entry postal-address -count 10
```

## Syntax of BNF files

We are trying to support [BNF](https://en.wikipedia.org/wiki/Backus%E2%80%93Naur_form) and [ABNF](https://en.wikipedia.org/wiki/Augmented_Backus%E2%80%93Naur_form) syntaxes simultenously, by allowing to use different syntactical elements for the same constructions. For example you can use `/` and `|` for [Rule Alternatives](https://en.wikipedia.org/wiki/Augmented_Backus%E2%80%93Naur_form#Alternative) and even mix them up in the same file. Both of them are interpreted as aliternatives.

*Maybe with some limitations we can enable support for [EBNF](https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form) as well, but it's a bit difficult because EBNF uses `;` to indicate the end of the rule definition, but ABNF and BNF use it for [comments](https://en.wikipedia.org/wiki/Augmented_Backus%E2%80%93Naur_form#Comment).*

*The descriptions below are stolen from wikipedia.*

### Comments

```lisp
; comment
```

*For some reason I also added C-style comments. Maybe I should remove them so to not create even more confusion between BNF dialects...*

```c
// comment
```

### Concatenation

```lisp
fu = %x61 ; a
bar = %x62 ; b
mumble = fu bar fu
```

### Alternative

```lisp
fubar = fu / bar
```

or

```lisp
fubar = fu | bar
```

### Incremental alternatives

The rule

```lisp
ruleset = alt1 / alt2
ruleset =/ alt3
ruleset =/ alt4 / alt5
```

is equivalent to

```lisp
ruleset = alt1 / alt2 / alt3 / alt4 / alt5
```

*Maybe to maintain the consistency in supporting mixed up syntax, we should allow to use `=|` along wit `=/`...*

### Value range

```lisp
OCTAL = %x30-37
```

is equivalent to

```lisp
OCTAL = "0" / "1" / "2" / "3" / "4" / "5" / "6" / "7"
```

and also can be written as

```lisp
OCTAL = "0" ... "9"
```

or

```lisp
OCTAL = "\x30" ... "\x37"
```

### Sequence group

```lisp
group = a (b / c) d
```

### Variable repetition

```lisp
n*nRule
```

### Specific repetition

```lisp
nRule
```

### Optional sequence

```lisp
[Rule]
```
