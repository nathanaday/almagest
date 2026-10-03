package hooks

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// shellCommands splits a command line into its simple commands, each a list of words, as
// bash and zsh read them before they run: quotes and escapes resolved, redirects and their
// targets taken out. A quoted word that holds a command of its own (sh -c "…", eval) is
// read as one more command.
func shellCommands(line string) [][]string {
	return splitCommands(line, 4)
}

func splitCommands(line string, depth int) [][]string {
	var (
		cmds   [][]string
		cur    []string
		word   strings.Builder
		inWord bool
		quoted bool // the word holds quoted text, so it may hold a command
		drop   bool // the next word is a redirect's target
	)
	endWord := func() {
		if !inWord {
			return
		}
		w := word.String()
		switch {
		case drop:
			drop = false
		default:
			cur = append(cur, w)
			if quoted && depth > 0 && strings.ContainsAny(w, " \t\n;&|()`<>") {
				cmds = append(cmds, splitCommands(w, depth-1)...)
			}
		}
		word.Reset()
		inWord, quoted = false, false
	}
	endCommand := func() {
		endWord()
		if len(cur) > 0 {
			cmds = append(cmds, cur)
		}
		cur = nil
	}
	at := func(i int) byte {
		if i < len(line) {
			return line[i]
		}
		return 0
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\':
			if at(i+1) == '\n' {
				i++
				continue
			}
			if i+1 < len(line) {
				word.WriteByte(line[i+1])
				i++
			}
			inWord = true
		case c == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				end = len(line) - i - 1
			}
			word.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord, quoted = true, true
		case c == '"':
			j := i + 1
			for ; j < len(line) && line[j] != '"'; j++ {
				if line[j] == '\\' && j+1 < len(line) && strings.IndexByte("$`\"\\\n", line[j+1]) >= 0 {
					if line[j+1] != '\n' {
						word.WriteByte(line[j+1])
					}
					j++
					continue
				}
				word.WriteByte(line[j])
			}
			i = j
			inWord, quoted = true, true
		case c == '$' && at(i+1) == '\'':
			j := i + 2
			for ; j < len(line) && line[j] != '\''; j++ {
				if line[j] == '\\' {
					j++
				}
			}
			word.WriteString(decodeANSI(line[i+2 : min(j, len(line))]))
			i = j
			inWord, quoted = true, true
		case c == '$' && at(i+1) == '"':
			// Locale quoting: the quotes that follow are plain double quotes.
		case c == ' ' || c == '\t':
			endWord()
		case c == '#' && !inWord:
			for i < len(line) && line[i] != '\n' {
				i++
			}
			endCommand()
		case c == '&' && at(i+1) == '>':
			// &> and &>> send both outputs to the word that follows.
			endWord()
			i++
			if at(i+1) == '>' {
				i++
			}
			drop = true
		case c == '<' || c == '>':
			// A word of digits, or {name} (zsh and bash 4), before the operator is its
			// file descriptor; any other word ends where the operator begins.
			w := word.String()
			if inWord && !quoted && (isDigits(w) || strings.HasPrefix(w, "{") && strings.HasSuffix(w, "}")) {
				word.Reset()
				inWord = false
			} else {
				endWord()
			}
			for _, op := range []string{"<<<", "<<-", "<<", "<>", ">>", ">|", ">&", "<&", "<", ">"} {
				if strings.HasPrefix(line[i:], op) {
					i += len(op) - 1
					break
				}
			}
			drop = true
		case strings.IndexByte(";&|()\n`", c) >= 0:
			endCommand()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand()
	return cmds
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

// decodeANSI is the text of $'…' quoting: its escapes as bash and zsh decode them.
func decodeANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			out.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case 'a', 'b', 'e', 'E', 'f', 'v':
			out.WriteByte(map[byte]byte{'a': 7, 'b': 8, 'e': 27, 'E': 27, 'f': 12, 'v': 11}[c])
		case 'x', 'u', 'U':
			limit := map[byte]int{'x': 2, 'u': 4, 'U': 8}[c]
			j := i + 1
			for j < len(s) && j-i-1 < limit && strings.IndexByte("0123456789abcdefABCDEF", s[j]) >= 0 {
				j++
			}
			if n, err := strconv.ParseUint(s[i+1:j], 16, 32); err == nil {
				if c == 'x' {
					out.WriteByte(byte(n))
				} else if utf8.ValidRune(rune(n)) {
					out.WriteRune(rune(n))
				}
			}
			i = j - 1
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i
			for j < len(s) && j-i < 3 && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			n, _ := strconv.ParseUint(s[i:j], 8, 8)
			out.WriteByte(byte(n))
			i = j - 1
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}
