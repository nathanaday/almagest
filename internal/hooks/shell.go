package hooks

import (
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// runners are the commands that run a string they are given as a command of its own, so
// a quoted word or a heredoc they take is read as one.
var runners = []string{"sh", "bash", "zsh", "dash", "ksh", "fish", "eval", "ssh", "su", "watch"}

// shellCommands splits a command line into its simple commands, each a list of words, as
// bash and zsh read them before they run: quotes and escapes resolved, braces expanded,
// redirects and their targets taken out, and a process substitution read as a command
// of its own. A quoted word or a heredoc that a shell or eval runs is read as one more
// command; any other quoted text is only text.
func shellCommands(line string) [][]string {
	return splitCommands(line, 4)
}

// heredoc is a here-document a command waits for: its delimiter, whether <<- strips the
// tabs before each line, and whether a runner reads it.
type heredoc struct {
	delim string
	tabs  bool
	run   bool
}

func splitCommands(line string, depth int) [][]string {
	var (
		cmds    [][]string
		cur     []string
		word    strings.Builder
		inWord  bool
		quoted  bool // the word holds quoted or escaped text, so it may hold a command
		literal bool // the word holds text a quote kept from brace expansion
		drop    bool // the next word is a redirect's target
		doc     *heredoc
		pending []heredoc
	)
	// A runner anywhere before the word counts: sudo sh -c, xargs sh -c, find -exec sh -c.
	runs := func() bool {
		return slices.ContainsFunc(cur, func(w string) bool { return slices.Contains(runners, path.Base(w)) })
	}
	endWord := func() {
		if !inWord {
			return
		}
		w := word.String()
		switch {
		case doc != nil:
			doc.delim = w
			pending = append(pending, *doc)
			doc = nil
		case drop:
			drop = false
		default:
			if quoted && depth > 0 && runs() && strings.ContainsAny(w, " \t\n;&|()`<>") {
				cmds = append(cmds, splitCommands(w, depth-1)...)
			}
			if literal {
				cur = append(cur, w)
			} else {
				cur = append(cur, expandBraces(w)...)
			}
		}
		word.Reset()
		inWord, quoted, literal = false, false, false
	}
	endCommand := func() {
		endWord()
		if len(cur) > 0 {
			cmds = append(cmds, cur)
		}
		cur, drop, doc = nil, false, nil
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
				if strings.IndexByte(" \t;&|()<>", line[i+1]) >= 0 {
					quoted = true
				}
				word.WriteByte(line[i+1])
				i++
			}
			inWord, literal = true, true
		case c == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				end = len(line) - i - 1
			}
			word.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord, quoted, literal = true, true, true
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
			inWord, quoted, literal = true, true, true
		case c == '$' && at(i+1) == '\'':
			j := i + 2
			for ; j < len(line) && line[j] != '\''; j++ {
				if line[j] == '\\' {
					j++
				}
			}
			word.WriteString(decodeANSI(line[i+2 : min(j, len(line))]))
			i = j
			inWord, quoted, literal = true, true, true
		case c == '$' && at(i+1) == '"':
			// Locale quoting: the quotes that follow are plain double quotes.
		case (c == '<' || c == '>') && at(i+1) == '(':
			// A process substitution runs a command of its own.
			endCommand()
			i++
		case c == ' ' || c == '\t':
			endWord()
		case c == '#' && !inWord:
			for i < len(line) && line[i] != '\n' {
				i++
			}
			i--
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
			op := ""
			for _, o := range []string{"<<<", "<<-", "<<", "<>", ">>", ">|", ">&", "<&", "<", ">"} {
				if strings.HasPrefix(line[i:], o) {
					op = o
					break
				}
			}
			i += len(op) - 1
			if op == "<<" || op == "<<-" {
				doc = &heredoc{tabs: op == "<<-", run: runs()}
			} else {
				drop = true
			}
		case c == '\n':
			endCommand()
			// The lines after a heredoc's command are its body, up to its delimiter.
			for _, h := range pending {
				var body []string
				for i+1 < len(line) {
					end := strings.IndexByte(line[i+1:], '\n')
					if end < 0 {
						end = len(line) - i - 1
					}
					l := line[i+1 : i+1+end]
					i += end + 1
					if h.tabs {
						l = strings.TrimLeft(l, "\t")
					}
					if l == h.delim {
						break
					}
					body = append(body, l)
				}
				if h.run && depth > 0 {
					cmds = append(cmds, splitCommands(strings.Join(body, "\n"), depth-1)...)
				}
			}
			pending = nil
		case strings.IndexByte(";&|()`", c) >= 0:
			endCommand()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand()
	return cmds
}

// expandBraces is a word with its brace lists expanded, as bash and zsh do: a{b,c}d is
// abd and acd. A word with no list is itself.
func expandBraces(w string) []string {
	start, end := -1, -1
	level := 0
	comma := false
	for i := 0; i < len(w); i++ {
		switch w[i] {
		case '{':
			if level == 0 && (i == 0 || w[i-1] != '$') {
				start, comma = i, false
			}
			if start >= 0 {
				level++
			}
		case ',':
			if level == 1 {
				comma = true
			}
		case '}':
			if start < 0 {
				continue
			}
			level--
			if level == 0 {
				if comma {
					end = i
					break
				}
				start = -1
			}
		}
		if end >= 0 {
			break
		}
	}
	if start < 0 || end < 0 {
		return []string{w}
	}
	var parts []string
	level, from := 0, start+1
	for i := start + 1; i < end; i++ {
		switch w[i] {
		case '{':
			level++
		case '}':
			level--
		case ',':
			if level == 0 {
				parts = append(parts, w[from:i])
				from = i + 1
			}
		}
	}
	parts = append(parts, w[from:end])
	var out []string
	for _, p := range parts {
		for _, e := range expandBraces(w[:start] + p + w[end+1:]) {
			if e != "" {
				out = append(out, e)
			}
		}
	}
	return out
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
			from := i + 1
			braced := c == 'x' && from < len(s) && s[from] == '{'
			if braced {
				from++
				limit = 8
			}
			j := from
			for j < len(s) && j-from < limit && strings.IndexByte("0123456789abcdefABCDEF", s[j]) >= 0 {
				j++
			}
			if n, err := strconv.ParseUint(s[from:j], 16, 32); err == nil {
				switch {
				case braced && n > 0x7f:
					if utf8.ValidRune(rune(n)) {
						out.WriteRune(rune(n))
					}
				case c == 'x':
					out.WriteByte(byte(n))
				case utf8.ValidRune(rune(n)):
					out.WriteRune(rune(n))
				}
			}
			if braced && j < len(s) && s[j] == '}' {
				j++
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
