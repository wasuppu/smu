# smu

simple markup - markdown like syntax, based on [smu](https://github.com/Gottox/smu) rewritten in Go

## Overview

I learned about smu from [Hacking on "smu", a Minimal Markdown Parser](https://www.karl.berlin/smu.html), but instead of using the
forked version, I rewrote the original version in Go without adding or removing any features, just added Unicode support.
The usage example below is a small demo in the cmd directory, demonstrating how to use it."

## Usage

```
Usage: smu [OPTION] ... [FILE]
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
          server port
```
