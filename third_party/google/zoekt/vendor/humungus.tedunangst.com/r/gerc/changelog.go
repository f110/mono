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

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (repo *Repo) readchangelog() (*Revlog, error) {
	if repo.changelog != nil {
		return repo.changelog, nil
	}
	revlog, err := repo.readrevlog("store/00changelog.i")
	if err != nil {
		return nil, err
	}
	repo.changelog = revlog
	repo.changes = make([]*Change, revlog.Len())
	tags := make(map[string]int)
	reverse := make(map[int][]string)
	tagdata, err := ioutil.ReadFile(repo.rootdir + "/.hgtags")
	if err == nil {
		lines := strings.Split(string(tagdata), "\n")
		for _, l := range lines {
			x := strings.Split(l, " ")
			if len(x) == 2 {
				rev, tag := x[0], x[1]
				idx := repo.hashtoidx(rev)
				if idx != -1 {
					tags[tag] = idx
				}
			}
		}
		for tag, idx := range tags {
			reverse[idx] = append(reverse[idx], tag)
		}
	}
	if idx := revlog.Len() - 1; idx > 0 {
		tags["tip"] = idx
		reverse[idx] = append(reverse[idx], "tip")
	}
	repo.tags = tags
	repo.reversetags = reverse
	return revlog, nil
}

// One changelog entry
type Change struct {
	Linkrev int
	NodeID  [32]byte
	P1rev   int
	P1node  [32]byte
	P2rev   int
	P2node  [32]byte
	ID      string
	User    string
	Date    time.Time
	Files   []string
	Summary string
	Message []string
	Tags    []string
	Diff    string
}

// Format and print one change
func (change *Change) Print(w io.Writer, verbose bool) {
	fmt.Fprintf(w, "changeset:   %d:%.6x\n", change.Linkrev, change.NodeID)
	if change.P1rev != change.Linkrev-1 {
		fmt.Fprintf(w, "parent:      %d:%.6x\n", change.P1rev, change.P1node)
	}
	if change.P2rev != -1 {
		fmt.Fprintf(w, "parent:      %d:%.6x\n", change.P2rev, change.P2node)
	}
	for _, t := range change.Tags {
		fmt.Fprintf(w, "tag:         %s\n", t)
	}
	fmt.Fprintf(w, "user:        %s\n", change.User)
	fmt.Fprintf(w, "date:        %s\n", change.Date.String())
	if verbose {
		fmt.Fprintf(w, "files:       %s\n", strings.Join(change.Files, " "))
		fmt.Fprintf(w, "description:\n%s\n\n", strings.Join(change.Message, "\n"))
	} else {
		fmt.Fprintf(w, "summary:     %s\n", change.Summary)
	}
	if change.Diff != "" {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "%s", change.Diff)
	}
}

func (repo *Repo) getchange(idx int) (*Change, error) {
	revlog, err := repo.readchangelog()
	if err != nil {
		return nil, err
	}
	if idx < 0 || idx >= revlog.Len() {
		return nil, RevisionNotFoundError{}
	}
	change := repo.changes[idx]
	if change != nil {
		return change, nil
	}
	data, _, err := revlog.GetData(idx)
	if err != nil {
		return nil, err
	}
	rev, err := revlog.Get(idx)
	if err != nil {
		return nil, err
	}
	change = new(Change)
	change.Linkrev = rev.Linkrev
	change.NodeID = rev.NodeID
	change.P1rev = rev.P1rev
	if change.P1rev != -1 {
		p1, _ := revlog.Get(change.P1rev)
		if p1 != nil {
			change.P1node = p1.NodeID
		}
	}
	change.P2rev = rev.P2rev
	if change.P2rev != -1 {
		p2, _ := revlog.Get(change.P2rev)
		if p2 != nil {
			change.P2node = p2.NodeID
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return nil, fmt.Errorf("no line")
	}
	change.ID = scanner.Text()
	if !scanner.Scan() {
		return nil, fmt.Errorf("no line")
	}
	change.User = scanner.Text()
	if !scanner.Scan() {
		return nil, fmt.Errorf("no line")
	}
	dt := strings.Split(scanner.Text(), " ")
	if len(dt) < 2 {
		return nil, fmt.Errorf("bad date")
	}
	secs, _ := strconv.ParseInt(dt[0], 10, 0)
	//tz, _ := strconv.ParseInt(dt[1], 10, 0)
	change.Date = time.Unix(secs, 0)

	var files []string
	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			break
		}
		files = append(files, l)
	}
	// already sorted?
	sort.Strings(files)
	change.Files = files

	var msg []string
	for scanner.Scan() {
		l := scanner.Text()
		msg = append(msg, l)
	}
	if len(msg) == 0 {
		return nil, fmt.Errorf("missing message for rev %d", idx)
	}
	change.Summary = msg[0]
	change.Message = msg
	change.Tags = repo.reversetags[idx]
	repo.changes[idx] = change
	return change, nil
}

// One file in the manifest
type ManifestFile struct {
	Name     string
	Revision string
	Flags    byte
}

func parsemanifest(revlog *Revlog, idx int) ([]*ManifestFile, error) {
	data, _, err := revlog.GetData(idx)
	if err != nil {
		return nil, err
	}
	var files []*ManifestFile
	lines := bytes.Split(data, []byte("\n"))
	for _, l := range lines[0 : len(lines)-1] {
		x := bytes.SplitN(l, []byte("\x00"), 2)
		if len(x) != 2 {
			return nil, fmt.Errorf("corrupt manifest for rev %d", idx)
		}
		name, hash := x[0], x[1]
		mf := &ManifestFile{
			Name:     string(name),
			Revision: string(hash),
		}
		files = append(files, mf)
	}
	return files, nil
}

// time.Time.Round works in UTC time, so we must make some offset adjustments
// before and after to get rounding in the local timezone.
// returns a nil slice if it's not a date.
// returns a possibly empty not nil slice if it's a date.
// returns an error if the date was incorrectly formatted.
func (repo *Repo) tryparsedate(arg string) ([]int, error) {
	revlog, _ := repo.readchangelog()
	l := revlog.Len()
	switch arg {
	case "date(today)":
		rv := make([]int, 0)
		now := time.Now()
		_, offset := now.Zone()
		now = now.Add(time.Duration(offset) * time.Second)
		start := now.Add(-12 * time.Hour).Round(24 * time.Hour)
		start = start.Add(time.Duration(-offset) * time.Second)
		end := now.Add(12 * time.Hour).Round(24 * time.Hour)
		end = end.Add(time.Duration(-offset) * time.Second)
		for i := 0; i < l; i++ {
			change, _ := repo.getchange(i)
			if change.Date.After(start) && change.Date.Before(end) {
				rv = append(rv, i)
			}
		}
		return rv, nil
	case "date(yesterday)":
		rv := make([]int, 0)
		now := time.Now()
		_, offset := now.Zone()
		now = now.Add(time.Duration(offset) * time.Second)
		start := now.Add(-36 * time.Hour).Round(24 * time.Hour)
		start = start.Add(time.Duration(-offset) * time.Second)
		end := now.Add(-12 * time.Hour).Round(24 * time.Hour)
		end = end.Add(time.Duration(-offset) * time.Second)
		for i := 0; i < l; i++ {
			change, _ := repo.getchange(i)
			if change.Date.After(start) && change.Date.Before(end) {
				rv = append(rv, i)
			}
		}
		return rv, nil
	}
	return nil, nil
}

// have to be a bit careful. this function is called inside readchangelog.
// the changelog is saved in the repo at that point, but the tag map is not.
func (repo *Repo) hashtoidx(hash string) int {
	revlog, _ := repo.readchangelog()
	if repo.tags != nil {
		idx, ok := repo.tags[hash]
		if ok {
			return idx
		}
	}
	if len(hash) > 40 || len(hash) < 12 {
		return -1
	}
	idx := -1
	for i, rev := range revlog.revs {
		h := fmt.Sprintf("%.20x", rev.NodeID)
		if strings.HasPrefix(h, hash) {
			if idx == -1 {
				idx = i
			} else {
				// too many matches
				return -1
			}
		}
	}
	return idx
}

func (repo *Repo) parseonerevnum(arg string) (int, error) {
	x := strings.Split(arg, "~")
	if len(x) > 1 {
		n, err := repo.parseonerevnum(x[0])
		if err != nil {
			return -1, err
		}
		an, err := strconv.Atoi(x[1])
		return n - an, nil
	}
	revlog, _ := repo.readchangelog()
	idx := repo.hashtoidx(arg)
	if idx != -1 {
		return idx, nil
	}

	n, err := strconv.Atoi(arg)
	if err != nil {
		return -1, RevisionNotFoundError{}
	}
	if n < 0 {
		lastrev := revlog.Len()
		return lastrev + n, nil
	}
	return n, nil
}

func (repo *Repo) parserevnum(arg string) ([]int, error) {
	revlog, err := repo.readchangelog()
	if err != nil {
		return nil, err
	}
	re_limit := regexp.MustCompile(`limit\((.*),\s*(\d+)\s*\)`)
	match := re_limit.FindStringSubmatch(arg)
	if match != nil {
		rv, err := repo.parserevnum(match[1])
		if err != nil {
			return nil, err
		}
		limit, err := strconv.Atoi(match[2])
		if err != nil {
			return nil, err
		}
		if len(rv) > limit {
			rv = rv[0:limit]
		}
		return rv, nil
	}
	re_limit = regexp.MustCompile(`last\((.*),\s*(\d+)\s*\)`)
	match = re_limit.FindStringSubmatch(arg)
	if match != nil {
		rv, err := repo.parserevnum(match[1])
		if err != nil {
			return nil, err
		}
		limit, err := strconv.Atoi(match[2])
		if err != nil {
			return nil, err
		}
		if len(rv) > limit {
			l := len(rv)
			rv = rv[l-limit:]
		}
		return rv, nil
	}

	rv, err := repo.tryparsedate(arg)
	if err != nil {
		return nil, err
	}
	if rv != nil {
		return rv, nil
	}

	x := strings.Split(arg, ":")
	if len(x) > 2 {
		return nil, fmt.Errorf("too many revisions in range")
	}
	if len(x) > 1 {
		var s, e int
		var err error
		if len(x[0]) > 0 {
			rv, err := repo.tryparsedate(x[0])
			if err != nil {
				return nil, err
			}
			if rv != nil {
				if len(rv) == 0 {
					return rv, nil
				}
				s = rv[0]
			} else {
				s, err = repo.parseonerevnum(x[0])
				if err != nil {
					return nil, err
				}
			}
		} else {
			s = 0
		}
		if len(x[1]) > 0 {
			rv, err := repo.tryparsedate(x[1])
			if err != nil {
				return nil, err
			}
			if rv != nil {
				if len(rv) == 0 {
					return rv, nil
				}
				e = rv[len(rv)-1]
			} else {
				e, err = repo.parseonerevnum(x[1])
				if err != nil {
					return nil, err
				}
			}
		} else {
			e = revlog.Len() - 1
		}
		if s < e {
			for i := s; i <= e; i++ {
				rv = append(rv, i)
			}
		} else {
			for i := s; i >= e; i-- {
				rv = append(rv, i)
			}
		}
		return rv, err
	} else {
		s, err := repo.parseonerevnum(x[0])
		if err != nil {
			return nil, err
		}
		return []int{s}, nil
	}
}
