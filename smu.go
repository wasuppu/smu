package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"unicode"
	"unicode/utf8"
)

const (
	BUFSIZ      = 1024
	VERSION     = "1.0"
	codeFence   = "```"
	htmlComment = "<!--"
)

type Tag struct {
	search  string
	process int
	before  string
	after   string
}

type Parser func(text []byte, newblock bool) (affected int)

var (
	noHTML      bool
	inParagraph bool
	pEndRegex   *regexp.Regexp
	parsers     []Parser
	lineprefixs []Tag
	underlines  []Tag
	surrounds   []Tag
	replaces    [][2]string
	alignTable  []string
)

func init() {
	pEndRegex = regexp.MustCompile("(\n\n|(^|\n)```)")

	lineprefixs = []Tag{
		{"    ", 0, "<pre><code>", "\n</code></pre>"},
		{"\t", 0, "<pre><code>", "\n</code></pre>"},
		{">", 2, "<blockquote>", "</blockquote>"},
		{"###### ", 1, "<h6>", "</h6>"},
		{"##### ", 1, "<h5>", "</h5>"},
		{"#### ", 1, "<h4>", "</h4>"},
		{"### ", 1, "<h3>", "</h3>"},
		{"## ", 1, "<h2>", "</h2>"},
		{"# ", 1, "<h1>", "</h1>"},
		{"- - -\n", 1, "<hr />", ""},
		{"---\n", 1, "<hr />", ""},
	}

	underlines = []Tag{
		{"=", 1, "<h1>", "</h1>\n"},
		{"-", 1, "<h2>", "</h2>\n"},
	}

	surrounds = []Tag{
		{"```", 0, "<code>", "</code>"},
		{"``", 0, "<code>", "</code>"},
		{"`", 0, "<code>", "</code>"},
		{"___", 1, "<strong><em>", "</em></strong>"},
		{"***", 1, "<strong><em>", "</em></strong>"},
		{"__", 1, "<strong>", "</strong>"},
		{"**", 1, "<strong>", "</strong>"},
		{"_", 1, "<em>", "</em>"},
		{"*", 1, "<em>", "</em>"},
	}

	replaces = [][2]string{
		{"\\\\", "\\"},
		{"\\`", "`"},
		{"\\*", "*"},
		{"\\_", "_"},
		{"\\{", "{"},
		{"\\}", "}"},
		{"\\[", "["},
		{"\\]", "]"},
		{"\\(", "("},
		{"\\)", ")"},
		{"\\#", "#"},
		{"\\+", "+"},
		{"\\-", "-"},
		{"\\.", "."},
		{"\\!", "!"},
		{"\\\"", "&quot;"},
		{"\\$", "$"},
		{"\\%", "%"},
		{"\\&", "&amp;"},
		{"\\'", "'"},
		{"\\,", ","},
		{"\\-", "-"},
		{"\\.", "."},
		{"\\/", "/"},
		{"\\:", ":"},
		{"\\;", ";"},
		{"\\<", "&lt;"},
		{"\\>", "&gt;"},
		{"\\=", "="},
		{"\\?", "?"},
		{"\\@", "@"},
		{"\\^", "^"},
		{"\\|", "|"},
		{"\\~", "~"},
		{"<", "&lt;"},
		{">", "&gt;"},
		{"&amp;", "&amp;"},
		{"&", "&amp;"},
		{"  \n", "<br />\n"},
	}

	parsers = []Parser{
		dounderline,
		docomment,
		docodefence,
		dolineprefix,
		dolist,
		dotable,
		doparagraph,
		dosurround,
		dolink,
		doshortlink,
		dohtml,
		doreplace,
	}

	alignTable = []string{
		"",
		" style=\"text-align: left\"",
		" style=\"text-align: right\"",
		" style=\"text-align: center\"",
	}
}

func endParagraph() {
	if inParagraph {
		fmt.Fprint(os.Stdout, "</p>\n")
		inParagraph = false
	}
}

func docomment(text []byte, newblock bool) int {
	begin, end := 0, len(text)
	if noHTML || !bytes.HasPrefix(text[begin:], []byte(htmlComment)) {
		return 0
	}
	p := bytes.Index(text[begin:], []byte("-->"))

	if p == -1 || p+3 > end {
		return 0
	}
	fmt.Fprintf(os.Stdout, "%s\n", text[begin:][:p+3])
	return (p + 3) * map[bool]int{true: -1, false: 1}[newblock]
}

func docodefence(text []byte, newblock bool) int {
	begin, end := 0, len(text)
	l := len(codeFence)

	if !newblock {
		return 0
	}

	if !bytes.HasPrefix(text[begin:], []byte(codeFence)) {
		return 0
	}

	/* Find start of content and read language string */
	start := begin + l
	langStart := start
	for start < end && text[start] != '\n' {
		start++
	}
	langStop := start
	start++

	/* Find end of fence */
	p := start - 1
	var stop int
	for {
		stop = p
		idx := bytes.Index(text[p+1:], []byte(codeFence))
		if idx == -1 {
			p = end
			break
		}
		p += 1 + idx
		if p >= len(text) || (p > 0 && text[p-1] != '\\') {
			stop = p
			break
		}
	}

	/* No closing code fence means the rest of file is code (CommonMark) */
	if p >= len(text) {
		stop = end
	}

	/* Print output */
	if langStart == langStop {
		fmt.Fprint(os.Stdout, "<pre><code>")
	} else {
		fmt.Fprint(os.Stdout, "<pre><code class=\"language-")
		hprint(text[langStart:langStop])
		fmt.Fprintln(os.Stdout, "\">")
	}
	hprint(text[start:stop])
	fmt.Fprint(os.Stdout, "</code></pre>\n")
	return -(stop - begin + l)
}

func dohtml(text []byte, newblock bool) int {
	begin, end := 0, len(text)

	if noHTML || begin+2 >= end {
		return 0
	}
	p := begin
	if text[p] != '<' || !isAlpha(text[p+1]) {
		return 0
	}

	p++
	tagStart := p
	for p < end && isAlnum(text[p]) {
		p++
	}
	tagend := p
	if p > end || tagStart == tagend {
		return 0
	}
	tag := string(text[tagStart:tagend])
	closeTag := []byte("</" + tag + ">")
	closeIdx := bytes.Index(text[p:], closeTag)
	if closeIdx != -1 {
		fmt.Fprintf(os.Stdout, "%s", text[begin:p+closeIdx+len(closeTag)])
		return p + closeIdx + len(closeTag)
	}

	closeIdx = bytes.IndexByte(text[tagend:], '>')
	if closeIdx != -1 {
		fmt.Fprintf(os.Stdout, "%s", text[begin:tagend+closeIdx+1])
		return tagend + closeIdx + 1
	}

	return 0
}

func dolineprefix(text []byte, newBlock bool) int {
	begin, end := 0, len(text)

	var p, consumedInput int
	if newBlock {
		p = begin
	} else if text[begin] == '\n' {
		p = begin + 1
		consumedInput += 1
	} else {
		return 0
	}

	for _, lineprefix := range lineprefixs {
		l := len(lineprefix.search)
		if end-p+1 < l {
			continue
		}
		if !bytes.HasPrefix(text[p:], []byte(lineprefix.search)) {
			continue
		}

		if text[begin] == '\n' {
			fmt.Fprint(os.Stdout, "\n")
		}

		/* All line prefixes add a block element. These are not allowed
		 * inside paragraphs, so we must end the paragraph first. */
		endParagraph()

		fmt.Fprint(os.Stdout, lineprefix.before)
		if lineprefix.search[l-1] == '\n' {
			fmt.Fprint(os.Stdout, "\n")
			return l - 1 + consumedInput
		}

		/* Collect lines into buffer while they start with the prefix */
		var buffer bytes.Buffer
		var j int
		for bytes.HasPrefix(text[p:], []byte(lineprefix.search)) && p+l < end {
			p += l

			/* Special case for blockquotes: optional space after > */
			if lineprefix.search[0] == '>' && text[p] == ' ' {
				p++
			}

			newline := bytes.IndexByte(text[p:], '\n')
			if newline == -1 {
				n, _ := buffer.Write(text[p:])
				j += n
				p += n
			} else {
				j += newline + 1
				buffer.Write(text[p : p+newline+1])
				p += newline + 1
			}
		}

		/* Skip empty lines in block */
		bs := buffer.Bytes()
		for j > 0 && j < len(bs) && bs[j] == '\n' {
			j--
		}

		bs = bs[:j]
		if lineprefix.process > 0 {
			process(bs, lineprefix.process >= 2)
		} else {
			hprint(bs)
		}
		fmt.Fprintln(os.Stdout, lineprefix.after)
		return -(p - begin)
	}
	return 0
}

func dolink(text []byte, newBlock bool) int {
	begin, end := 0, len(text)
	parensDepth := 1

	var img bool
	if text[begin] == '[' {
		img = false
	} else if bytes.HasPrefix(text[begin:], []byte("![")) {
		img = true
	} else {
		return 0
	}

	desc := 1
	p := desc
	if img {
		desc = 2
		p = desc
	}

	if idx := bytes.Index(text[desc:], []byte("](")); idx == -1 || p+idx > end {
		return 0
	} else {
		p += idx
	}

	descend := p
	link := p + 2

	/* find end of link while handling nested parens */
	q := link
	for parensDepth != 0 {
		idx := bytes.IndexAny(text[q:], "()")
		if idx == -1 {
			return 0
		}
		q += idx
		if text[q] == '(' {
			parensDepth++
		} else {
			parensDepth--
		}
		if parensDepth != 0 && q < end {
			q++
		}
	}

	var linkend int
	title, titleend := -1, -1
	if idx := bytes.IndexAny(text[link:], "\"'"); idx != -1 && link+idx < end && q > link+idx {
		p = link + idx
		sep := text[p] /* separator: can be " or ' */
		title = p + 1
		/* strip trailing whitespace */
		linkend = p
		for linkend > link && isSpace(text[linkend-1]) {
			linkend--
		}
		titleend = q - 1
		for titleend > link && isSpace(text[titleend]) {
			titleend--
		}
		if titleend < title || text[titleend] != sep {
			return 0
		}

	} else {
		linkend = q
	}

	/* Links can be given in angular brackets */
	if text[link] == '<' && text[linkend-1] == '>' {
		link++
		linkend--
	}

	l := q + 1 - begin
	if img {
		fmt.Fprint(os.Stdout, "<img src=\"")
		hprint(text[link:linkend])
		fmt.Fprint(os.Stdout, "\" alt=\"")
		hprint(text[desc:descend])
		fmt.Fprint(os.Stdout, "\" ")
		if title != -1 && titleend != -1 {
			fmt.Fprint(os.Stdout, "title=\"")
			hprint(text[title:titleend])
			fmt.Fprint(os.Stdout, "\" ")
		}
		fmt.Fprint(os.Stdout, "/>")
	} else {
		fmt.Fprint(os.Stdout, "<a href=\"")
		hprint(text[link:linkend])
		fmt.Fprint(os.Stdout, "\"")
		if title != -1 && titleend != -1 {
			fmt.Fprint(os.Stdout, " title=\"")
			hprint(text[title:titleend])
			fmt.Fprint(os.Stdout, "\"")
		}
		fmt.Fprint(os.Stdout, ">")
		process(text[desc:descend], false)
		fmt.Fprint(os.Stdout, "</a>")
	}
	return l
}

func dolist(text []byte, newBlock bool) int {
	begin, end := 0, len(text)

	var p int
	if newBlock {
		p = begin
	} else if text[begin] == '\n' {
		p = begin + 1
	} else {
		return 0
	}

	q := p
	var marker byte
	var numStart, startNumber int
	if text[p] == '-' || text[p] == '*' || text[p] == '+' {
		marker = text[p]
	} else {
		numStart = p
		for p < end && isDigit(text[p]) {
			p++
		}
		if p >= end || (text[p] != '.' && text[p] != ')') {
			return 0
		}
		startNumber, _ = strconv.Atoi(string(text[numStart:p]))
	}
	p++
	if p >= end || !isSpace(text[p]) {
		return 0
	}

	endParagraph()
	p++
	for p != end && isSpace(text[p]) {
		p++
	}
	ident := p - q
	if !newBlock {
		fmt.Fprint(os.Stdout, "\n")
	}

	if marker != 0 {
		fmt.Fprint(os.Stdout, "<ul>\n")
	} else if startNumber == 1 {
		fmt.Fprint(os.Stdout, "<ol>\n")
	} else {
		fmt.Fprintf(os.Stdout, "<ol start=\"%d\">\n", startNumber)
	}

	var buffer bytes.Buffer
	isBlock := 0
	var j int
	for run := true; p < end && run; p++ {
		buffer.Reset()
		for i := 0; p < end && run; p, i = p+1, i+1 {
			if text[p] == '\n' {
				if p+1 == end {
					break
				} else {
					/* Handle empty lines */
					for q = p + 1; q < end && isSpace(text[q]); q++ {
					}
					if q < end && text[q] == '\n' {
						buffer.WriteByte('\n')
						i++
						run = false
						isBlock++
						p = q
					}
				}
				q = p + 1
				j = 0
				if marker != 0 && q < end && text[q] == marker {
					j = 1
				} else {
					for q+j < end && isDigit(text[q+j]) && j < ident {
						j++
					}
					if q+j >= end {
						break
					}
					if j > 0 && (text[q+j] == '.' || text[q+j] == ')') {
						j++
					} else {
						j = 0
					}
				}
				if q+ident < end {
					for j < ident && q+j < end && isSpace(text[q+j]) {
						j++
					}
				}
				if j == ident {
					buffer.WriteByte('\n')
					i++
					p += ident
					run = true
					if q < end && isSpace(text[q]) {
						p++
					} else {
						break
					}
				} else if j < ident {
					run = false
				}
			}
			buffer.WriteByte(text[p])
		}
		fmt.Fprint(os.Stdout, "<li>")
		bs := buffer.Bytes()
		process(bs, isBlock > 1 || (isBlock == 1 && run))
		fmt.Fprint(os.Stdout, "</li>\n")
	}
	if marker != 0 {
		fmt.Fprint(os.Stdout, "</ul>\n")
	} else {
		fmt.Fprint(os.Stdout, "</ol>\n")
	}
	p--
	p--
	for p > begin && text[p] == '\n' {
		p--
	}
	return -(p - begin + 1)
}

var intable, inrow, incell int
var calign int64

func dotable(text []byte, newBlock bool) int {
	begin, end := 0, len(text)

	l := 8 * 4 // sizeof(calign) * 4

	var p int
	if text[begin] != '|' {
		return 0
	}
	if intable == 2 { /* in alignment row, skip it. */
		intable++
		p = begin
		for p < end && text[p] != '\n' {
			p++
		}
		return p - begin + 1
	}

	if inrow != 0 && (begin+1 >= end || text[begin+1] == '\n') { /* close cell and row and if ends, table too */
		if inrow == -1 {
			fmt.Fprintf(os.Stdout, "</t%c></tr>", 'h')
		} else {
			fmt.Fprintf(os.Stdout, "</t%c></tr>", 'd')
		}
		if inrow == -1 {
			intable = 2
		}
		inrow = 0
		if end-begin <= 2 || text[begin+2] == '\n' {
			intable = 0
			fmt.Fprint(os.Stdout, "\n</table>\n")
		}
		return 1
	}

	if intable == 0 { /* open table */
		intable = 1
		inrow = -1
		incell = 0
		calign = 0
		p = begin
		for p < end && text[p] != '\n' {
			p++
		}
		if p < end && text[p] == '\n' { /* load alignment from 2nd line */
			for i, p := -1, p+1; p < end && text[p] != '\n'; p++ {
				if text[p] == '|' {
					i++
					for p+1 < end && isSpace(text[p+1]) {
						p++
					}
					if i < l && p+1 < end && text[p+1] == ':' {
						calign |= 1 << (i * 2)
					}
					if p+1 < end && text[p+1] == '\n' {
						break
					}
				} else if i < l && text[p] == ':' {
					calign |= 1 << (i*2 + 1)
				}
			}
			fmt.Fprint(os.Stdout, "<table>\n<tr>")
		}
	}

	/* open row */
	if inrow == 0 {
		inrow = 1
		incell = 0
		fmt.Fprint(os.Stdout, "<tr>")
	}

	typ := 'd'
	if inrow == -1 {
		typ = 'h'
	}

	/* close cell */
	if incell != 0 {
		fmt.Fprintf(os.Stdout, "</t%c>", typ)
	}

	/* open cell */
	align := 0
	if incell < l {
		align = int((calign >> (incell * 2)) & 3)
	}

	fmt.Fprintf(os.Stdout, "<t%c%s>", typ, alignTable[align])
	incell++
	for p = begin + 1; p < end && isSpace(text[p]); p++ {
	}
	return p - begin
}

func doparagraph(text []byte, newBlock bool) int {
	begin, end := 0, len(text)

	if !newBlock {
		return 0
	}

	var p int
	if match := pEndRegex.FindIndex(text[begin+1:]); match == nil {
		p = end
	} else {
		p = begin + 1 + match[0]
	}

	fmt.Fprint(os.Stdout, "<p>")
	inParagraph = true
	process(text[begin:p], false)
	endParagraph()

	return -(p - begin)
}

func doreplace(text []byte, newBlock bool) int {
	begin, end := 0, len(text)

	for _, replace := range replaces {
		l := len(replace[0])
		if end-begin < l {
			continue
		}
		if bytes.HasPrefix(text[begin:begin+l], []byte(replace[0])) {
			fmt.Fprint(os.Stdout, replace[1])
			return l
		}
	}
	return 0
}

func doshortlink(text []byte, newBlock bool) int {
	begin, end := 0, len(text)
	var ismall int

	if text[begin] != '<' {
		return 0
	}
	for p := begin + 1; p != end; p++ {
		switch text[p] {
		case ' ', '\t', '\n':
			return 0
		case '#', ':':
			ismall = -1
		case '@':
			if ismall == 0 {
				ismall = 1
			}
		case '>':
			if ismall == 0 {
				return 0
			}
			fmt.Fprint(os.Stdout, "<a href=\"")
			if ismall == 1 {
				fmt.Fprint(os.Stdout, "&#x6D;&#x61;i&#x6C;&#x74;&#x6F;:")
				for c := begin + 1; c < p; c++ {
					fmt.Fprintf(os.Stdout, "&#%d;", text[c])
				}
				fmt.Fprint(os.Stdout, "\">")
				for c := begin + 1; c < p; c++ {
					fmt.Fprintf(os.Stdout, "&#%d;", text[c])
				}
			} else {
				hprint(text[begin+1 : p])
				fmt.Fprint(os.Stdout, "\">")
				hprint(text[begin+1 : p])
			}
			fmt.Fprint(os.Stdout, "</a>")
			return p - begin + 1
		}
	}
	return 0
}

func dosurround(text []byte, newBlock bool) int {
	begin, end := 0, len(text)
	for _, surround := range surrounds {
		l := len(surround.search)
		if end-begin < 2*l || !bytes.HasPrefix(text[begin:], []byte(surround.search)) {
			continue
		}
		start := begin + l
		p := start
		var stop int

		for p < end {
			idx := bytes.Index(text[p:], []byte(surround.search))
			if idx == -1 {
				break
			}
			stop = p + idx

			if stop > start && text[stop-1] == '\\' {
				p = stop + 1
				continue
			}
			break
		}

		if stop < start || stop >= end {
			continue
		}

		fmt.Fprint(os.Stdout, surround.before)

		/* Single space at start and end are ignored */
		if start < stop && text[start] == ' ' && text[stop-1] == ' ' && start < stop-1 {
			start++
			stop--
		}

		if surround.process > 0 {
			process(text[start:stop], false)
		} else {
			hprint(text[start:stop])
		}
		fmt.Fprint(os.Stdout, surround.after)
		return stop - begin + l
	}
	return 0
}

func dounderline(text []byte, newBlock bool) int {
	begin, end := 0, len(text)
	if !newBlock {
		return 0
	}
	p := begin
	l := 0
	for p+l < end && text[p+l] != '\n' {
		l++
	}
	p += l + 1
	if l == 0 || p >= end {
		return 0
	}

	for _, underline := range underlines {
		j := 0
		for p+j < end && text[p+j] != '\n' && text[p+j] == underline.search[0] {
			j++
		}

		if j >= 3 {
			fmt.Fprint(os.Stdout, underline.before)
			if underline.process > 0 {
				process(text[:l], false)
			} else {
				hprint(text[:l])
			}
			fmt.Fprint(os.Stdout, underline.after)
			return -(j + p - begin)
		}
	}
	return 0
}

func hprint(text []byte) {
	for len(text) > 0 {
		r, size := utf8.DecodeRune(text)
		if r == utf8.RuneError {
			break
		}

		switch r {
		case '&':
			fmt.Fprint(os.Stdout, "&amp;")
		case '"':
			fmt.Fprint(os.Stdout, "&quot;")
		case '>':
			fmt.Fprint(os.Stdout, "&gt;")
		case '<':
			fmt.Fprint(os.Stdout, "&lt;")
		default:
			fmt.Fprintf(os.Stdout, "%c", r)
		}
		text = text[size:]
	}
}

func process(text []byte, newblock bool) {
	begin, end := 0, len(text)
	for p := begin; p < end; {
		if newblock {
			for p < len(text) && text[p] == '\n' {
				p++
				if p == end {
					return
				}
			}
		}

		affected := 0
		for _, parser := range parsers {
			affected = parser(text[p:end], newblock)
			if affected != 0 {
				break
			}
		}

		if affected != 0 {
			p += abs(affected)
		} else {
			if text[p] < utf8.RuneSelf {
				fmt.Fprintf(os.Stdout, "%c", text[p])
				p++
			} else {
				r, size := utf8.DecodeRune(text[p:])
				if r != utf8.RuneError {
					fmt.Fprintf(os.Stdout, "%c", r)
					p += size
				} else {
					fmt.Fprintf(os.Stdout, "%c", text[p])
					p++
				}
			}
		}

		/* Don't print single newline at end */
		if p+1 == end && text[p] == '\n' {
			return
		}

		if p < len(text) && text[p] == '\n' && p+1 < end && text[p+1] == '\n' {
			newblock = true
		} else {
			newblock = affected < 0
		}
	}
}

func main() {
	md, _ := os.ReadFile(os.Args[1])
	process(md, true)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func isDigit(c byte) bool {
	r, _ := utf8.DecodeRune([]byte{c})
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsDigit(r)
}

func isAlpha(c byte) bool {
	r, _ := utf8.DecodeRune([]byte{c})
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r) || r == '_'
}

func isAlnum(c byte) bool {
	r, _ := utf8.DecodeRune([]byte{c})
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t'
}
