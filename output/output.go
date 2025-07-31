package output

import (
	"fmt"
	"io"
)

// Printer interface for outputting strings and stringers
type Printer interface {
	Print(str string)
	Printf(format string, a ...any) Printer
}

// writer implements Printer for real output
type writer struct {
	w io.Writer
}

func (w *writer) Print(str string) {
	fmt.Fprint(w.w, str)
}

func (w *writer) Printf(format string, a ...any) Printer {
	fmt.Fprintf(w.w, format, a...)
	return w
}

// New creates a printer that outputs to the given io.Writer
func New(w io.Writer) Printer {
	return &writer{w: w}
}

type Matcher interface {
	Match(string) bool
}
