package edit

import (
	"fmt"
	"unicode/utf8"
)

// Completion subsystem.

// Interface.

type completion struct {
	completer  string
	candidates []*candidate
	selected   int
	lines      int
}

func (*completion) Mode() ModeType {
	return modeCompletion
}

func (c *completion) ModeLine(width int) *buffer {
	return makeModeLine(fmt.Sprintf("COMPLETING %s", c.completer), width)
}

func startCompletion(ed *Editor) {
	startCompletionInner(ed, false)
}

func completePrefixOrStartCompletion(ed *Editor) {
	startCompletionInner(ed, true)
}

func selectCandUp(ed *Editor) {
	ed.completion.prev(false)
}

func selectCandDown(ed *Editor) {
	ed.completion.next(false)
}

func selectCandLeft(ed *Editor) {
	if c := ed.completion.selected - ed.completion.lines; c >= 0 {
		ed.completion.selected = c
	}
}

func selectCandRight(ed *Editor) {
	if c := ed.completion.selected + ed.completion.lines; c < len(ed.completion.candidates) {
		ed.completion.selected = c
	}
}

func cycleCandRight(ed *Editor) {
	ed.completion.next(true)
}

// acceptCompletion accepts currently selected completion candidate.
func acceptCompletion(ed *Editor) {
	c := ed.completion
	if 0 <= c.selected && c.selected < len(c.candidates) {
		accepted := c.candidates[c.selected].source.text
		ed.insertAtDot(accepted)
	}
	ed.mode = &ed.insert
}

func defaultCompletion(ed *Editor) {
	acceptCompletion(ed)
	ed.nextAction = action{typ: reprocessKey}
}

// Implementation.

type styled struct {
	text  string
	style string
}

type candidate struct {
	source, menu styled
	// XXX only used in completers for compound.
	sourceSuffix string
}

func (c *completion) prev(cycle bool) {
	c.selected--
	if c.selected == -1 {
		if cycle {
			c.selected = len(c.candidates) - 1
		} else {
			c.selected++
		}
	}
}

func (c *completion) next(cycle bool) {
	c.selected++
	if c.selected == len(c.candidates) {
		if cycle {
			c.selected = 0
		} else {
			c.selected--
		}
	}
}

func startCompletionInner(ed *Editor, completePrefix bool) {
	token := tokenAtDot(ed)
	node := token.Node
	if node == nil {
		return
	}

	c := &completion{}
	for _, compl := range completers {
		candidates := compl.completer(node, ed)
		if candidates != nil {
			c.completer = compl.name
			c.candidates = candidates
			break
		}
	}

	if c.candidates == nil {
		ed.addTip("unsupported completion :(")
	} else if len(c.candidates) == 0 {
		ed.addTip("no candidate for %s", c.completer)
	} else {
		if completePrefix {
			// If there is a non-empty longest common prefix, insert it and
			// don't start completion mode.
			// As a special case, when there is exactly one candidate, it is
			// immeidately accepted.
			prefix := c.candidates[0].source.text
			for _, cand := range c.candidates[1:] {
				prefix = commonPrefix(prefix, cand.source.text)
			}
			if prefix != "" {
				ed.insertAtDot(prefix)
				return
			}
		}
		ed.completion = *c
		ed.mode = &ed.completion
	}
}

var badToken = Token{}

func tokenAtDot(ed *Editor) Token {
	if len(ed.tokens) == 0 || ed.dot > len(ed.line) {
		return badToken
	}
	if ed.dot == len(ed.line) {
		return ed.tokens[len(ed.tokens)-1]
	}
	for _, token := range ed.tokens {
		if ed.dot < token.Node.End() {
			return token
		}
	}
	return badToken
}

func commonPrefix(s, t string) string {
	for i, r := range s {
		if i >= len(t) {
			return s[:i]
		}
		r2, _ := utf8.DecodeRuneInString(t[i:])
		if r2 != r {
			return s[:i]
		}
	}
	return s
}

const completionListingColMargin = 2

func (comp *completion) List(width, maxHeight int) *buffer {
	b := newBuffer(width)
	// Layout candidates in multiple columns
	cands := comp.candidates

	// First decide the shape (# of rows and columns)
	colWidth := 0
	margin := completionListingColMargin
	for _, cand := range cands {
		width := WcWidths(cand.menu.text)
		if colWidth < width {
			colWidth = width
		}
	}

	cols := (b.width + margin) / (colWidth + margin)
	if cols == 0 {
		cols = 1
	}
	lines := CeilDiv(len(cands), cols)
	comp.lines = lines

	// Determine the window to show.
	low, high := findWindow(lines, comp.selected%lines, maxHeight)
	for i := low; i < high; i++ {
		if i > low {
			b.newline()
		}
		for j := 0; j < cols; j++ {
			k := j*lines + i
			if k >= len(cands) {
				break
			}
			style := cands[k].menu.style
			if k == comp.selected {
				style += styleForSelected
			}
			text := cands[k].menu.text
			if j > 0 {
				b.writePadding(margin, "")
			}
			b.writes(ForceWcWidth(text, colWidth), style)
		}
	}
	return b
}
