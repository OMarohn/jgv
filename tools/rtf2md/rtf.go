package main

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	cellSep = "\x1f"
	rowSep  = "\x1e"
)

var cp1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
	0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘',
	0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—', 0x98: '˜',
	0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

var skipDestinations = map[string]bool{
	"fonttbl": true, "colortbl": true, "stylesheet": true, "info": true,
	"pict": true, "header": true, "footer": true, "themedata": true,
	"colorschememapping": true, "latentstyles": true, "datastore": true,
	"listtable": true, "listoverridetable": true, "rsidtbl": true,
	"generator": true, "xmlnstbl": true, "wgrffmtfilter": true,
	"fldinst": true, "private": true, "mmathPr": true, "pgptbl": true,
	"revtbl": true, "operator": true, "falt": true, "panose": true,
}

// rtfToText converts RTF to plain text. Cells end with cellSep, rows with rowSep, paragraphs with \n.
func rtfToText(src []byte) string {
	var out strings.Builder
	type state struct{ skip bool }
	stack := []state{{}}
	skip := func() bool { return stack[len(stack)-1].skip }
	uc := 1
	skipChars := 0

	i := 0
	for i < len(src) {
		c := src[i]
		switch c {
		case '{':
			stack = append(stack, stack[len(stack)-1])
			i++
		case '}':
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			i++
		case '\r', '\n':
			i++
		case '\\':
			i++
			if i >= len(src) {
				break
			}
			n := src[i]
			switch {
			case n == '\'':
				if i+2 < len(src) {
					v, err := strconv.ParseUint(string(src[i+1:i+3]), 16, 8)
					if err == nil && !skip() {
						if skipChars > 0 {
							skipChars--
						} else if r, ok := cp1252[byte(v)]; ok {
							out.WriteRune(r)
						} else {
							out.WriteRune(rune(v))
						}
					}
				}
				i += 3
			case n == '*':
				stack[len(stack)-1].skip = true
				i++
			case n == '\\' || n == '{' || n == '}':
				if !skip() {
					out.WriteByte(n)
				}
				i++
			case n == '~':
				if !skip() {
					out.WriteByte(' ')
				}
				i++
			case n == '-' || n == '_':
				i++
			case (n >= 'a' && n <= 'z') || (n >= 'A' && n <= 'Z'):
				j := i
				for j < len(src) && ((src[j] >= 'a' && src[j] <= 'z') || (src[j] >= 'A' && src[j] <= 'Z')) {
					j++
				}
				word := string(src[i:j])
				k := j
				if k < len(src) && (src[k] == '-' || (src[k] >= '0' && src[k] <= '9')) {
					k++
					for k < len(src) && src[k] >= '0' && src[k] <= '9' {
						k++
					}
				}
				param := string(src[j:k])
				i = k
				if i < len(src) && src[i] == ' ' {
					i++
				}
				if skipDestinations[word] {
					stack[len(stack)-1].skip = true
				}
				if skip() {
					break
				}
				switch word {
				case "par", "line":
					out.WriteByte('\n')
				case "tab":
					out.WriteByte('\t')
				case "cell":
					out.WriteString(cellSep)
				case "row":
					out.WriteString(rowSep)
				case "uc":
					if v, err := strconv.Atoi(param); err == nil {
						uc = v
					}
				case "u":
					if v, err := strconv.Atoi(param); err == nil {
						if v < 0 {
							v += 65536
						}
						out.WriteRune(rune(v))
						skipChars = uc
					}
				case "endash":
					out.WriteString("–")
				case "emdash":
					out.WriteString("—")
				}
			default:
				i++
			}
		default:
			if !skip() {
				if skipChars > 0 {
					skipChars--
				} else {
					r, size := utf8.DecodeRune(src[i:])
					if r == utf8.RuneError && size == 1 {
						out.WriteRune(rune(c))
					} else {
						out.WriteRune(r)
					}
					i += size
					continue
				}
			}
			i++
		}
	}
	return out.String()
}
