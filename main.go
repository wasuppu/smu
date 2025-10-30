package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

const (
	defaultTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.title}}</title>
    <style>{{.css}}</style>
</head>
<body>
    {{.body}}
</body>
</html>`
	defaultCss = `
body {
  font-family: sans-serif;
  font-size: 16px;
  text-size-adjust: none;
  max-width: 680px;
  margin: 30px auto 0 auto;
}
@media (max-width: 980px) {
  body {
    max-width: 90%;
    font-size: 2em;
  }
}
h1, h2, h3, h4, h5, h6 {
  margin-bottom: 0.5em;
}
h1 {
  font-size: 48px;
  text-align: center;
}
h2 {
  border-bottom: 3px black solid;
}
h1 > a, h2 > a {
  text-decoration: none;
}
a:hover {
  opacity: 0.5;
}
p, ul {
  margin: 0 auto 0.5em auto;
}
code {
  background: #eee;
  padding: 0.3rem;
  tab-size: 4;
}
pre code {
  display: block;
  overflow-x: auto;
  padding: 0.3rem 0.6rem;
}
`
)

var (
	tpl       *template.Template
	tplbuffer bytes.Buffer
	tplpath   string
	csspath   string
)

func main() {
	flag.BoolVar(&noHTML, "n", false, "no html")
	interactive := flag.Bool("i", false, "interactive mode")
	outpath := flag.String("o", "", "output file path")
	flag.StringVar(&tplpath, "t", "default", "template file path")
	flag.StringVar(&csspath, "css", "default", "css file path")
	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of smu:\n")
		flag.PrintDefaults()
	}

	var infile *os.File
	if *interactive {
		infile = os.Stdin
	} else if flag.NArg() == 0 {
		flag.Usage()
		return
	} else {
		file, err := os.Open(flag.Arg(0))
		must(err)
		infile = file
	}

	text, err := io.ReadAll(infile)
	must(err)
	if flagVisited("t") {
		must(processTemplate(text))
		if *outpath == "-" {
			fmt.Print(tplbuffer.String())
		} else {
			os.WriteFile(*outpath, tplbuffer.Bytes(), 0644)
		}
	} else {
		process(text, true)
		if *outpath == "" {
			fmt.Print(outbuffer.String())
		} else {
			os.WriteFile(*outpath, outbuffer.Bytes(), 0644)
		}
	}
}

func processTemplate(text []byte) (err error) {
	process(text, true)
	body := outbuffer.String()
	title := extractTitle(body)

	if tplpath == "default" {
		tpl = template.Must(template.New("markdown").Parse(defaultTemplate))
	} else {
		tpl, err = template.ParseFiles(tplpath)
		if err != nil {
			return err
		}
	}

	var css string
	if csspath == "default" {
		css = defaultCss
	} else {
		bs, err := os.ReadFile(csspath)
		if err != nil {
			return err
		}
		css = string(bs)
	}

	m := map[string]string{
		"title": title,
		"css":   css,
		"body":  body,
	}

	return tpl.Execute(&tplbuffer, m)
}

func extractTitle(text string) string {
	if h1Start := strings.Index(text, "<h1>"); h1Start != -1 {
		h1End := strings.Index(text[h1Start:], "</h1>")
		if h1End != -1 {
			title := text[h1Start+4 : h1Start+h1End]
			return strings.TrimSpace(title)
		}
	}
	return ""
}

func flagVisited(name string) bool {
	visited := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			visited = true
		}
	})
	return visited
}

func must(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
