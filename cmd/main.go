package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/wasuppu/smu"
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
	tplpath   = "default"
	csspath   = "default"
	port      = 8080
)

func main() {
	var (
		err         error
		infile      *os.File
		outpath     string
		useTemplate bool
		server      bool
		interactive bool
	)

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--no-html":
			smu.NoHTML = true
		case "-o", "--output":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				outpath = args[i+1]
				i++
			}
		case "-t", "--template":
			useTemplate = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				tplpath = args[i+1]
				i++
			}
		case "-css", "--stylesheet":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				csspath = args[i+1]
				i++
			}
		case "-s", "--server":
			server = true
		case "-p", "--port":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				port, err = strconv.Atoi(args[i+1])
				must(err)
				i++
			}
		case "-i", "--interactive":
			interactive = true
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
				os.Exit(1)
			} else if infile == nil {
				file, err := os.Open(args[i])
				must(err)
				infile = file
			}
		}
	}

	if interactive {
		infile = os.Stdin
	} else if infile == nil {
		Usage()
		return
	}

	text, err := io.ReadAll(infile)
	must(err)
	if server {
		must(processTemplate(text))
		runserver()
		return
	}

	if useTemplate {
		must(processTemplate(text))
		writeOutput(outpath, tplbuffer.Bytes())
	} else {
		result := smu.Process(text)
		writeOutput(outpath, result)
	}
}

func writeOutput(outpath string, result []byte) {
	if outpath == "" {
		fmt.Print(string(result))
	} else {
		os.WriteFile(outpath, result, 0644)
	}
}

func processTemplate(text []byte) (err error) {
	body := string(smu.Process(text))
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

func runserver() {
	fmt.Printf("Started server on http://localhost:%d\n", port)
	http.HandleFunc("/", serveMarkdown)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func serveMarkdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(tplbuffer.Bytes())
}

func must(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func Usage() {
	usage := `Usage: smu [OPTION] ... [FILE]
    -n, --no-html         no html
    -i, --interactive     interactive mode
    -o, --output          string
          output file path
    -t, --template         string
          template file path (default "default")
    -css, --stylesheet     string
          css file path (default "default")
    -s, --server           start server
    -p, --port             int
          server port`
	fmt.Println(usage)
}
