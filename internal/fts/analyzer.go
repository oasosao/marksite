package fts

import (
	"unicode"
	"unicode/utf8"

	"github.com/blugelabs/bluge/analysis"
	"github.com/blugelabs/bluge/analysis/token"
)

// cjkAnalyzer 英文按词切分、中文按单字切分的简易分析器（无第三方词典依赖）
var cjkAnalyzer = &analysis.Analyzer{
	Tokenizer:    cjkTokenizer{},
	TokenFilters: []analysis.TokenFilter{token.NewLowerCaseFilter()},
}

type cjkTokenizer struct{}

func (cjkTokenizer) Tokenize(input []byte) analysis.TokenStream {
	tokens := analysis.TokenStream{}
	i := 0
	for i < len(input) {
		r, size := utf8.DecodeRune(input[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if isASCIIWord(r) {
			start := i
			for i < len(input) {
				r2, size2 := utf8.DecodeRune(input[i:])
				if !isASCIIWord(r2) {
					break
				}
				i += size2
			}
			tokens = append(tokens, &analysis.Token{
				Start: start, End: i, Term: input[start:i], PositionIncr: 1, Type: analysis.AlphaNumeric,
			})
			continue
		}
		start := i
		i += size
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			tokens = append(tokens, &analysis.Token{
				Start: start, End: i, Term: input[start:i], PositionIncr: 1, Type: analysis.Ideographic,
			})
		}
	}
	return tokens
}

func isASCIIWord(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
