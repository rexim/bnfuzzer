# Go BNF Fuzzer

Generate random messages based on their [BNF](https://en.wikipedia.org/wiki/Backus%E2%80%93Naur_form) definition.

## Quickl Start

Generate 10 random postal addresses:

```console
$ go run ./main.go -file postal.bnf -entry postal-address -count 10
```
