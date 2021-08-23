/*
 * Copyright (c) 2019 Ted Unangst <tedu@tedunangst.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package gerc

// This is an implementation of Bram Cohen's patience diffing algorithm.
// The concept is pretty simple. Pull matching lines off the head and tail
// of each array, then process the differing lines inside by finding unique
// matching lines. Next process each chunk between matching lines in
// the same way. Instead of recursing indefinitely, just fallback to the
// simplest diff possible: all old lines removed, all new lines added.
// We handle a few special cases when matching lines. Any number of blank
// lines can be processed before we start comparing lines for real.
// A peephole pass will try to recollect blank lines that get shuffled.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

type DiffLine struct {
	Line string
	Type byte // ' ', '+', or '-'
}

func newdiffline(t byte, l string) DiffLine {
	return DiffLine{Line: l, Type: t}
}

// Create a unified format diff.
func WriteUnidiff(w io.Writer, file1, date1 string, hasnl1 bool, file2, date2 string,
	hasnl2 bool, diffs []DiffLine) bool {

	hasdiff := false
	for i := range diffs {
		if diffs[i].Type != ' ' {
			hasdiff = true
			break
		}
	}
	if !hasdiff {
		return false
	}

	fmt.Fprintf(w, "--- %s\t%s\n", file1, date1)
	fmt.Fprintf(w, "+++ %s\t%s\n", file2, date2)
	offset1 := 0
	offset2 := 0
	for i := 0; i < len(diffs); i++ {
		x := diffs[i].Type
		if x != ' ' {
			clines := 0
			plines := 0
			mlines := 0
			ctx := 0
			end := i
			for j := i; j < len(diffs); j++ {
				y := diffs[j].Type
				end = j
				if y == ' ' {
					ctx++
					clines++
					if ctx == 7 {
						break
					}
				} else {
					ctx = 0
					if y == '+' {
						plines++
					} else {
						mlines++
					}
				}
			}
			start := i - 3
			clines += 3
			for start < 0 {
				clines--
				start++
			}
			for ctx > 3 {
				end--
				clines--
				ctx--
			}
			hint := ""
			for h := start; h >= 0; h-- {
				if len(diffs[h].Line) > 0 && !unicode.IsSpace(rune(diffs[h].Line[0])) {
					hint = " " + diffs[h].Line
					if len(hint) > 41 {
						hint = hint[:41]
					}
					break
				}
			}
			fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@%s\n",
				start+1-offset1, clines+mlines, start+1-offset2, clines+plines, hint)
			for j := start; j <= end; j++ {
				fmt.Fprintf(w, "%c%s\n", diffs[j].Type, diffs[j].Line)
			}
			offset1 += plines
			offset2 += mlines
			i = end
		}
	}
	// notyet
	if !hasnl1 {
		io.WriteString(w, "\\ No newline at end of file\n")
	}
	if !hasnl2 {
		io.WriteString(w, "\\ No newline at end of file\n")
	}
	return true
}

func bytestolines(d []byte) ([]string, bool) {
	hasnl := len(d) == 0 || len(d) > 0 && d[len(d)-1] == '\n'
	l := strings.Split(string(d), "\n")
	if hasnl {
		l = l[:len(l)-1]
	}
	return l, hasnl
}

func Unidiff(name1, date1 string, d1 []byte, name2, date2 string, d2 []byte) string {
	l1, hasnl1 := bytestolines(d1)
	l2, hasnl2 := bytestolines(d2)
	diffs := Diff(l1, l2)
	var buf strings.Builder
	WriteUnidiff(&buf, name1, date1, hasnl1, name2, date2, hasnl2, diffs)
	return buf.String()
}

// Create a diff using the patience algorithm.
func Diff(l1 []string, l2 []string) []DiffLine {
	// grab header first
	head, i1, i2 := matchlines(true, l1, 0, len(l1)-1, l2, 0, len(l2)-1)
	// pull off the tail next
	tail, j1, j2 := matchlines(false, l1, len(l1)-1, i1, l2, len(l2)-1, i2)
	// process the interior of the file
	diffs := interiordiff(true, l1[i1:j1+1], l2[i2:j2+1])
	// put it all together
	diffs = append(head, diffs...)
	diffs = append(diffs, tail...)
	return peephole(diffs)
}

func isboring(s string) bool {
	if s == "" {
		return true
	}
	if strings.TrimLeft(s, " \t") == "}" {
		return true
	}
	return false
}

// peephole optimizer...
// fix up a few cases where boring lines get subtracted and later added back
func peephole(diffs []DiffLine) []DiffLine {
	for i := 0; i < len(diffs); i++ {
		if diffs[i].Type == '-' && isboring(diffs[i].Line) {
			boring := diffs[i].Line
			state := 0
			blanks := 1
			for j := i + 1; j < len(diffs); j++ {
				if state == 0 && diffs[j].Type == '-' && diffs[j].Line == boring {
					blanks++
					continue
				}
				if (state == 0 || state == 1) &&
					diffs[j].Type == '+' && diffs[j].Line != boring {
					state = 1
					continue
				}
				if state == 1 && diffs[j].Type == ' ' && diffs[j].Line == boring {
					state = 2
					continue
				}
				if diffs[j].Type == '+' && diffs[j].Line == boring {
					b := 0
					for ; b < blanks && j+b < len(diffs) &&
						diffs[j+b].Type == '+' && diffs[j+b].Line == boring; b++ {
					}
					copy(diffs[i:], diffs[i+b:j])
					for n := 0; n < b; n++ {
						diffs[j-b+n].Type = ' '
						diffs[j-b+n].Line = boring
					}
					copy(diffs[j:], diffs[j+b:])
					diffs = diffs[:len(diffs)-b]
					i = j
				}
				break
			}
		}
		// add boring lines at the end of chunks, not the beginning
		if diffs[i].Type == '+' && isboring(diffs[i].Line) {
			boring := diffs[i].Line
			for j := i + 1; j < len(diffs); j++ {
				if diffs[j].Type == '+' {
					continue
				}
				if diffs[j].Type == ' ' && diffs[j].Line == boring {
					diffs[i].Type = ' '
					diffs[j].Type = '+'
				}
				if diffs[j].Type == '-' && diffs[j].Line == boring {
					b := 1
					diffs[i].Type = ' '
					diffs[i].Line = boring
					copy(diffs[j:], diffs[j+b:])
					diffs = diffs[:len(diffs)-b]
				}
				break
			}
		}
		// check that all sequences are subtractions followed by additions
		if diffs[i].Type == ' ' && i+1 < len(diffs) && diffs[i+1].Type == '+' {
			start := i + 1
			end := i + 1
			for j := i + 1; j < len(diffs); j++ {
				end = j
				if diffs[j].Type != '+' {
					break
				}
			}
			if diffs[end].Type == '-' {
				n := 0
				for j := end; j < len(diffs); j++ {
					if diffs[j].Type != '-' {
						break
					}
					n++
				}
				tmp := make([]DiffLine, n)
				copy(tmp[:], diffs[end:end+n])
				copy(diffs[start+n:end+n], diffs[start:end])
				copy(diffs[start:start+n], tmp[:])
			}
		}
	}
	return diffs
}

// process forward or backwards matching segments
// returns a set of diff lines, and the positions of the not matching lines
func matchlines(forw bool, l1 []string, i1, e1 int, l2 []string, i2, e2 int) ([]DiffLine, int, int) {
	var diffs []DiffLine
	var w1, w2 int
	if forw {
		// first chew off any number of blank lines from either side
		for w1 = 0; i1+w1 <= e1; w1++ {
			if l1[i1+w1] != "" {
				break
			}
		}
		for w2 = 0; i2+w2 <= e2; w2++ {
			if l2[i2+w2] != "" {
				break
			}
		}
		i1, i2 = i1+w1, i2+w2
		for w2 < w1 {
			diffs = append(diffs, newdiffline('-', ""))
			w1--
		}
		for w1 < w2 {
			diffs = append(diffs, newdiffline('+', ""))
			w2--
		}
		for w1 > 0 {
			diffs = append(diffs, newdiffline(' ', ""))
			w1--
		}
		// now look for actual matching lines
		for ; i1 <= e1 && i2 <= e2; i1, i2 = i1+1, i2+1 {
			if l1[i1] == l2[i2] {
				diffs = append(diffs, newdiffline(' ', l1[i1]))
			} else {
				break
			}
		}
	} else {
		// first chew off any number of blank lines from either side
		for w1 = 0; i1-w1 >= e1; w1++ {
			if l1[i1-w1] != "" {
				break
			}
		}
		for w2 = 0; i2-w2 >= e2; w2++ {
			if l2[i2-w2] != "" {
				break
			}
		}
		i1, i2 = i1-w1, i2-w2
		for w2 < w1 {
			diffs = append(diffs, newdiffline('-', ""))
			w1--
		}
		for w1 < w2 {
			diffs = append(diffs, newdiffline('+', ""))
			w2--
		}
		for w1 > 0 {
			diffs = append(diffs, newdiffline(' ', ""))
			w1--
		}
		// now look for actual matching lines
		for ; i1 >= e1 && i2 >= e2; i1, i2 = i1-1, i2-1 {
			if l1[i1] == l2[i2] {
				diffs = append(diffs, newdiffline(' ', l1[i1]))
			} else {
				break
			}
		}
		for i, j := 0, len(diffs)-1; i < j; i, j = i+1, j-1 {
			diffs[i], diffs[j] = diffs[j], diffs[i]
		}
	}
	return diffs, i1, i2
}

// process lines that are different
func differinglines(t byte, l []string, start, stop int) []DiffLine {
	var diffs []DiffLine
	for i := start; i <= stop; i++ {
		diffs = append(diffs, newdiffline(t, l[i]))
	}
	return diffs
}

type uniq struct {
	pos1 int
	pos2 int
	prev *uniq
}

// find lines that only occur once each in both l1 and l2
func findmatchinglines(l1 []string, l2 []string, uniqueonly bool) []*uniq {
	m := make(map[string]*uniq)
	for i, l := range l1 {
		x := m[l]
		if x == nil {
			x = new(uniq)
			x.pos1 = i
			x.pos2 = -2
			m[l] = x
		} else if uniqueonly {
			x.pos1 = -1
		}
	}
	for i, l := range l2 {
		x := m[l]
		if x != nil {
			if x.pos2 == -2 {
				x.pos2 = i
			} else if uniqueonly {
				x.pos2 = -1
			}
		}
	}
	var u []*uniq
	for _, x := range m {
		if x.pos1 >= 0 && x.pos2 >= 0 {
			u = append(u, x)
		}
	}
	sort.Slice(u, func(i, j int) bool { return u[i].pos1 < u[j].pos1 })
	return u
}

// patience sort the uniq lines to find an increasing sequence
func findsubsequence(u []*uniq) []*uniq {
	var stacks []*uniq
	for _, x := range u {
		i := sort.Search(len(stacks), func(n int) bool { return x.pos2 < stacks[n].pos2 })
		if i > 0 {
			x.prev = stacks[i-1]
		}
		if i == len(stacks) {
			stacks = append(stacks, x)
		} else {
			stacks[i] = x
		}
	}
	u = []*uniq{}
	for x := stacks[len(stacks)-1]; x != nil; x = x.prev {
		u = append(u, x)
	}
	for i, j := 0, len(u)-1; i < j; i, j = i+1, j-1 {
		u[i], u[j] = u[j], u[i]
	}
	return u
}

// attempt to find some matching lines as anchors and continue diffing
func interiordiff(recurse bool, l1 []string, l2 []string) []DiffLine {
	var u []*uniq
	if recurse {
		// first time, try looking for unique lines only
		u = findmatchinglines(l1, l2, true)
		if len(u) == 0 {
			u = findmatchinglines(l1, l2, false)
		}
	}
	if !recurse {
		// second round, take anything
		u = findmatchinglines(l1, l2, false)
	}
	if len(u) == 0 {
		return diffchunk(false, l1, 0, len(l1)-1, l2, 0, len(l2)-1)
	}
	u = findsubsequence(u)

	var s1, s2 int = 0, 0
	var diffs []DiffLine
	for _, x := range u {
		diffs = append(diffs, diffchunk(recurse, l1, s1, x.pos1, l2, s2, x.pos2)...)
		s1, s2 = x.pos1+1, x.pos2+1
	}
	// append diffs for whatever is left
	diffs = append(diffs, diffchunk(recurse, l1, s1, len(l1)-1, l2, s2, len(l2)-1)...)
	return diffs
}

func diffchunk(recurse bool, l1 []string, s1, e1 int, l2 []string, s2, e2 int) []DiffLine {
	// grab header first
	head, i1, i2 := matchlines(true, l1, s1, e1, l2, s2, e2)
	// pull off the tail next
	tail, j1, j2 := matchlines(false, l1, e1, i1, l2, e2, i2)
	if recurse {
		// we'll try recursing once
		diffs := interiordiff(false, l1[i1:j1+1], l2[i2:j2+1])
		head = append(head, diffs...)
	} else {
		// append diffs for whatever is left
		head = append(head, differinglines('-', l1, i1, j1)...)
		head = append(head, differinglines('+', l2, i2, j2)...)
	}
	// put it all together
	head = append(head, tail...)
	return head
}
