package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
)

// stdinIsTerminal reports whether stdin is attached to an interactive
// terminal. `kgraph serve --pick` refuses to run without one (piped/CI
// stdin has no user to answer the prompt) — this is the detectable
// non-TTY failure mode the picker and prune confirmation share.
func stdinIsTerminal() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// chooseFromList prints title followed by a numbered list of items to out,
// reads one line from in, and returns the 0-based index of the chosen item.
// Invalid input (EOF, non-numeric, out of range) is an error — the caller
// decides whether to retry or fail. Extracted so `serve --pick` and any
// future numbered pickers share one tested implementation.
func chooseFromList(out io.Writer, in io.Reader, title string, items []string) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("nothing to choose from")
	}
	fmt.Fprintln(out, title)
	for i, item := range items {
		fmt.Fprintf(out, "  %d) %s\n", i+1, item)
	}
	fmt.Fprintf(out, "choose [1-%d]: ", len(items))

	line, err := readLine(in)
	if err != nil {
		return -1, fmt.Errorf("reading selection: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return -1, fmt.Errorf("not a number: %q", strings.TrimSpace(line))
	}
	if n < 1 || n > len(items) {
		return -1, fmt.Errorf("choice %d out of range [1-%d]", n, len(items))
	}
	return n - 1, nil
}

// confirmPrompt asks a yes/no question (defaulting to no) and reports the
// answer. EOF is reported as an error so callers can distinguish "user said
// no" from "no user at all"; both should stop a destructive operation.
func confirmPrompt(out io.Writer, in io.Reader, question string) (bool, error) {
	fmt.Fprint(out, question)
	line, err := readLine(in)
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// readLine reads a single line (up to '\n') from in, stripping the trailing
// \r\n/\n. A byte-at-a-time read via bufio keeps exactly one prompt's worth
// of input consumption simple for interactive stdin.
func readLine(in io.Reader) (string, error) {
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err == io.EOF {
		if line != "" {
			return line, nil // last line without trailing newline
		}
		return "", io.EOF
	}
	return line, err
}
