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

// wrappers run the command that follows them, after their own options; the value each
// option of valueOptions takes is a word of its own.
var (
	wrappers     = []string{"sudo", "doas", "env", "xargs", "nice", "nohup", "time", "timeout", "stdbuf", "exec", "command", "builtin"}
	valueOptions = map[string][]string{
		"sudo":    {"-u", "-g", "-C", "-D", "-h", "-p", "-r", "-t", "-U"},
		"xargs":   {"-I", "-n", "-P", "-L", "-s", "-d", "-E", "-a"},
		"nice":    {"-n"},
		"timeout": {"-s", "-k"},
		"stdbuf":  {"-i", "-o", "-e"},
		"env":     {"-u", "-C"},
	}
)

// commandName is a word as a command name reads: its base name, in lower case, without
// zsh's leading =.
func commandName(w string) string {
	return strings.ToLower(path.Base(strings.TrimPrefix(w, "=")))
}

// runsString reports whether a simple command runs a string as a command: a runner where
// the command name stands, after assignments and wrappers, env -S, or a runner after
// find's -exec. A word that holds whitespace, such as a commit message, names nothing.
func runsString(words []string) bool {
	for i, w := range words {
		if (w == "-exec" || w == "-execdir" || w == "-ok") && i+1 < len(words) && slices.Contains(runners, commandName(words[i+1])) {
			return true
		}
	}
	i := 0
	for i < len(words) {
		w := words[i]
		switch name := commandName(w); {
		case strings.ContainsAny(w, " \t\n"):
			return false
		case strings.Contains(w, "=") && !strings.HasPrefix(w, "="):
			i++ // an assignment before the command
		case slices.Contains(runners, name):
			return true
		case slices.Contains(wrappers, name):
			i++
			for i < len(words) && strings.HasPrefix(words[i], "-") {
				if name == "env" && (words[i] == "--split-string" || strings.HasPrefix(words[i], "-S")) {
					return true
				}
				if slices.Contains(valueOptions[name], words[i]) {
					i++
				}
				i++
			}
			if name == "timeout" && i < len(words) {
				i++ // the duration
			}
		default:
			return false
		}
	}
	return false
}

// simple is one simple command and the pipeline it belongs to.
type simple struct {
	words []string
	pipe  int
}

// heredoc is a here-document a command waits for: its delimiter, whether <<- strips the
// tabs before each line, and whether a runner reads it.
type heredoc struct {
	delim string
	tabs  bool
	run   bool
}

// splitCommands splits a command line into its simple commands, each a list of words, as
// bash and zsh read them before they run: quotes and escapes resolved, braces expanded,
// redirects and their targets taken out, and a process substitution read as a command
// of its own. In a pipeline that runs a string (runsString: sh -c, eval, … | sh), every
// quoted word and heredoc is read as one more command, at most depth levels deep; in
// any other pipeline quoted text is only text.
func splitCommands(line string, depth int) []simple {
	// A first pass, which reads no quoted word, finds the pipelines that run a string.
	running := map[int]bool{}
	if depth > 0 {
		for _, c := range splitCommands(line, 0) {
			running[c.pipe] = running[c.pipe] || runsString(c.words)
		}
	}
	var (
		cmds    []simple
		pipe    int
		cur     []string
		word    strings.Builder
		inWord  bool
		quoted  bool // the word holds quoted or escaped text, so it may hold a command
		drop    bool // the next word is a redirect's target
		doc     *heredoc
		pending []heredoc
	)
	runs := func() bool { return running[pipe] }
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
				cmds = append(cmds, inner(splitCommands(w, depth-1))...)
			}
			cur = append(cur, expandBraces(w)...)
		}
		word.Reset()
		inWord, quoted = false, false
	}
	// endCommand ends a simple command; piped says the next one reads its output.
	endCommand := func(piped bool) {
		endWord()
		if len(cur) > 0 {
			cmds = append(cmds, simple{cur, pipe})
		}
		cur, drop, doc = nil, false, nil
		if !piped {
			pipe++
		}
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
		case (c == '<' || c == '>') && at(i+1) == '(':
			// A process substitution runs a command of its own.
			endCommand(false)
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
			endCommand(false)
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
					cmds = append(cmds, inner(splitCommands(strings.Join(body, "\n"), depth-1))...)
				}
			}
			pending = nil
		case c == '|' && at(i+1) == '|', c == '&' && at(i+1) == '&':
			endCommand(false)
			i++
		case c == '|':
			// | and |& pipe the command into the next one.
			endCommand(true)
			if at(i+1) == '&' {
				i++
			}
		case strings.IndexByte(";&()`", c) >= 0:
			endCommand(false)
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand(false)
	return cmds
}

// inner marks the commands read out of a quoted word or a heredoc as a pipeline of their
// own, apart from the line's.
func inner(cmds []simple) []simple {
	for i := range cmds {
		cmds[i].pipe = -1
	}
	return cmds
}

// expandBraces is a word with its brace lists expanded, as bash and zsh do: a{b,c}d is
// abd and acd. A word with no list is itself.
func expandBraces(w string) []string {
	if braceCount(w) > maxBraceWords {
		return []string{w}
	}
	return expandAll(w)
}

// braceCount is how many words a word's brace lists give, counted without expanding
// them, and more than maxBraceWords once it passes that.
func braceCount(w string) int {
	n := 1
	for i := 0; i < len(w); i++ {
		if w[i] != '{' || i > 0 && w[i-1] == '$' {
			continue
		}
		level, end, parts := 0, -1, 1
		for j := i; j < len(w) && end < 0; j++ {
			switch w[j] {
			case '{':
				level++
			case '}':
				level--
				if level == 0 {
					end = j
				}
			case ',':
				if level == 1 {
					parts++
				}
			}
		}
		if end < 0 {
			break
		}
		if r := braceRange(w[i+1 : end]); r != nil {
			parts = len(r)
		} else if inner := braceCount(w[i+1 : end]); inner > 1 {
			parts *= inner
		}
		n *= parts
		if n > maxBraceWords {
			return maxBraceWords + 1
		}
		i = end
	}
	return n
}

// expandAll expands every brace list of a word.
func expandAll(w string) []string {
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
				if comma || braceRange(w[start+1:i]) != nil {
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
	parts := braceRange(w[start+1 : end])
	level, from := 0, start+1
	if parts == nil {
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
	}
	var out []string
	for _, p := range parts {
		for _, e := range expandAll(w[:start] + p + w[end+1:]) {
			if e != "" {
				out = append(out, e)
			}
		}
	}
	return out
}

// maxBraceWords bounds a brace expansion.
const maxBraceWords = 256

// braceRange is the words of a {a..c} or {1..3} range, or nil when the text is no range.
func braceRange(text string) []string {
	lo, hi, ok := strings.Cut(text, "..")
	if !ok {
		return nil
	}
	if a, err := strconv.Atoi(lo); err == nil {
		b, err := strconv.Atoi(hi)
		if err != nil || b-a > 1000 || a-b > 1000 {
			return nil
		}
		var out []string
		for i := a; ; i += map[bool]int{true: 1, false: -1}[b >= a] {
			out = append(out, strconv.Itoa(i))
			if i == b {
				return out
			}
		}
	}
	if len(lo) != 1 || len(hi) != 1 || !isLetter(lo[0]) || !isLetter(hi[0]) {
		return nil
	}
	var out []string
	for c := lo[0]; ; c += map[bool]byte{true: 1, false: 255}[hi[0] >= lo[0]] {
		out = append(out, string(c))
		if c == hi[0] {
			return out
		}
	}
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

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
