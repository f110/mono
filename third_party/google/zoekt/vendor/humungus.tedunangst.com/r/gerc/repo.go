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

// gerc - good enough revision control
package gerc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Find the root directory for a repository, starting at start.
func FindRepo(start string) (string, error) {
	start, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("no current directory?")
	}
	for start != "/" {
		dothg := start + "/.hg"
		info, err := os.Stat(dothg)
		if err == nil {
			if info.IsDir() {
				return start, nil
			}
			return "", fmt.Errorf("%s is not a directory", dothg)
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		start = filepath.Dir(start)
	}
	return "", fmt.Errorf("could not find .hg")
}

// A handle to a repository
type Repo struct {
	rootdir     string
	changelog   *Revlog
	changes     []*Change
	cache       map[string]*Revlog
	fds         fdcache
	tags        map[string]int
	reversetags map[int][]string
}

// Open a repository located at path.
// This is the directory that contains .hg.
func Open(path string) (*Repo, error) {
	path = filepath.Clean(path)
	dothg := path + "/.hg"
	info, err := os.Stat(dothg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dothg)
	}
	return &Repo{
		rootdir: path,
		cache:   make(map[string]*Revlog),
		fds: fdcache{
			ents: make([]fdcacheent, 8),
		},
	}, nil
}

// Close a repository, free resources.
func (repo *Repo) Close() {
	repo.rootdir = "/invalid"
	repo.changelog = nil
	repo.changes = nil
	repo.cache = nil
	repo.fds.close()
	repo.tags = nil
	repo.reversetags = nil
}

// A file could not be found
type FileNotFoundError struct{}

func (e FileNotFoundError) Error() string {
	return "requested file could not be found"
}

// no going outside the root
func (repo *Repo) checkfilename(filename string) error {
	if filename == "" {
		return nil
	}
	if filename[0] == '/' {
		return FileNotFoundError{}
	}
	path := filepath.Clean(repo.rootdir + "/" + filename)
	if len(path) <= len(repo.rootdir) ||
		!strings.HasPrefix(path, repo.rootdir) ||
		path[len(repo.rootdir)] != '/' {
		return FileNotFoundError{}
	}
	// not allowed to dig into .hg either
	if (len(path) == len(repo.rootdir)+4 &&
		path == repo.rootdir+"/.hg") ||
		(len(path) > len(repo.rootdir)+4 &&
			strings.HasPrefix(path, repo.rootdir+"/.hg/")) {
		return FileNotFoundError{}
	}
	return nil
}

// Return an absolute path to a file in the filesystem
func (repo *Repo) FSPath(filename string) (string, error) {
	err := repo.checkfilename(filename)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s", repo.rootdir, filename)
	return path, nil
}

func storename(file string) string {
	var newname []byte
	for i, c := range []byte(file) {
		if i == 0 && c == '.' {
			newname = append(newname, '~')
			newname = append(newname, '2')
			newname = append(newname, 'e')
			continue
		}
		if c >= 'A' && c <= 'Z' {
			newname = append(newname, '_')
			c += 'a' - 'A'
		}
		if c == '_' {
			newname = append(newname, '_')
		}
		newname = append(newname, c)
	}
	return fmt.Sprintf("store/data/%s.i", newname)
}

func (repo *Repo) hgfilename(file string) string {
	path := fmt.Sprintf("%s/.hg/%s", repo.rootdir, file)
	return path
}

func (repo *Repo) hgopen(file string) (*os.File, error) {
	// better if checked by callers, but can check again
	err := repo.checkfilename(file)
	if err != nil {
		return nil, err
	}
	path := repo.hgfilename(file)
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return fd, nil
}

func (repo *Repo) getdiff(filename string, linkrev int) (string, error) {
	err := repo.checkfilename(filename)
	if err != nil {
		return "", err
	}
	revlog, err := repo.readrevlog(storename(filename))
	if err != nil {
		return "", err
	}
	idx := revlog.FindLink(linkrev)
	if idx == -1 {
		return filename + " was deleted\n", nil
	}

	rev, err := revlog.Get(idx)
	if err != nil {
		return "", err
	}
	change, err := repo.getchange(rev.Linkrev)
	if err != nil {
		return "", err
	}

	// collect data from previous changelog entry
	node := change.NodeID[:]
	var prevnode []byte
	if change.P1rev != -1 {
		prev, _ := repo.getchange(change.P1rev)
		prevnode = prev.NodeID[:]
	} else {
		prevnode = []byte{0, 0, 0, 0, 0, 0}
	}

	var date1 string
	date2 := change.Date.String()
	file1 := filename
	var prevdata []byte

	// collect data from previous file revision
	if rev.P1rev != -1 {
		prevdata, _, err = revlog.GetData(rev.P1rev)
		if err != nil {
			return "", err
		}
		prev, _ := repo.getchange(change.P1rev)
		date1 = prev.Date.String()
	} else {
		file1 = "/dev/null"
		date1 = time.Unix(0, 0).UTC().String()
	}

	data, _, err := revlog.GetData(idx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("diff -r %.6x -r %.6x %s\n", prevnode, node, filename) +
		Unidiff(file1, date1, prevdata, filename, date2, data), nil
}

func contains(array []string, s string) bool {
	idx := sort.SearchStrings(array, s)
	return idx < len(array) && array[idx] == s
}

type ChangesArgs struct {
	Revisions string
	Filename  string
	WithDiff  bool
}

// Get changes from the changelog.
func (repo *Repo) GetChanges(args ChangesArgs) ([]*Change, error) {
	var wantrevs []int
	if args.Revisions != "" {
		r, err := repo.parserevnum(args.Revisions)
		if err != nil {
			return nil, err
		}
		if wantrevs == nil {
			// make sure it's not nil even if r is empty
			wantrevs = make([]int, 0)
		}
		wantrevs = append(wantrevs, r...)
	}
	filename := args.Filename
	withdiff := args.WithDiff

	if filename != "" && wantrevs == nil {
		err := repo.checkfilename(filename)
		if err != nil {
			return nil, err
		}
		filelog, err := repo.readrevlog(storename(filename))
		if err != nil {
			return nil, err
		}
		for i := filelog.Len() - 1; i >= 0; i-- {
			filerev, err := filelog.Get(i)
			if err != nil {
				return nil, err
			}
			wantrevs = append(wantrevs, filerev.Linkrev)
		}
	}

	revlog, err := repo.readchangelog()
	if err != nil {
		return nil, err
	}
	if wantrevs == nil {
		for i := revlog.Len() - 1; i >= 0; i-- {
			wantrevs = append(wantrevs, i)
		}
	}
	var changes []*Change
	for _, i := range wantrevs {
		change, err := repo.getchange(i)
		if err != nil {
			return nil, err
		}
		if filename != "" && !contains(change.Files, filename) {
			continue
		}
		if withdiff {
			dupe := new(Change)
			*dupe = *change
			change = dupe
			var buf strings.Builder
			for _, cf := range change.Files {
				if filename != "" && cf != filename {
					continue
				}
				diff, err := repo.getdiff(cf, change.Linkrev)
				if err != nil {
					return nil, err
				}
				buf.WriteString(diff)
			}
			change.Diff = buf.String()
		}
		changes = append(changes, change)
	}
	return changes, nil
}

type AnnotateArgs struct {
	Filename  string
	Revisions string
}

// Annotate a file
func (repo *Repo) Annotate(args AnnotateArgs) ([]Annotation, error) {
	var wantrevs []int
	if args.Revisions != "" {
		r, err := repo.parserevnum(args.Revisions)
		if err != nil {
			return nil, err
		}
		if wantrevs == nil {
			// make sure it's not nil even if r is empty
			wantrevs = make([]int, 0)
		}
		wantrevs = append(wantrevs, r...)
	}
	filename := args.Filename
	if filename == "" {
		return nil, fmt.Errorf("need a filename")
	}
	err := repo.checkfilename(filename)
	if err != nil {
		return nil, err
	}
	if wantrevs != nil && len(wantrevs) == 0 {
		return nil, RevisionNotFoundError{}
	}
	if len(wantrevs) > 2 {
		return nil, fmt.Errorf("too many revisions")
	}
	filelog, err := repo.readrevlog(storename(filename))
	if err != nil {
		return nil, err
	}
	var start, end int
	if wantrevs == nil {
		start = 0
		end = filelog.Len() - 1
	}
	if len(wantrevs) == 1 {
		start = 0
		end = filelog.FindLink(wantrevs[0])
		// notyet
		want := wantrevs[0]
		for end == -1 && want > 0 {
			want--
			end = filelog.FindLink(want)
		}
	}
	if len(wantrevs) == 2 {
		start = filelog.FindLink(wantrevs[0])
		end = filelog.FindLink(wantrevs[1])
	}
	if start == -1 || end == -1 {
		return nil, RevisionNotFoundError{}
	}

	var annos []Annotation
	for i := start; i <= end; i++ {
		rev, _ := filelog.Get(i)
		change, _ := repo.getchange(rev.Linkrev)
		c := fmt.Sprintf("%.6x", change.NodeID)
		data, _, _ := filelog.GetData(i)
		annos = Annotate(annos, string(data), c)
	}
	return annos, nil
}

// A tag
type Tag struct {
	Name    string
	Linkrev int
	NodeID  [32]byte
}

// Get all the tags
func (repo *Repo) GetTags() []Tag {
	_, err := repo.readchangelog()
	if err != nil {
		return nil
	}
	var tags []Tag
	for k, v := range repo.tags {
		c, _ := repo.getchange(v)
		tags = append(tags, Tag{Name: k, Linkrev: c.Linkrev, NodeID: c.NodeID})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Linkrev == tags[j].Linkrev {
			return tags[i].Name > tags[j].Name
		}
		return tags[i].Linkrev > tags[j].Linkrev
	})
	return tags
}

type FilesArgs struct {
	Filenames []string
	Revision  string
}

// Get the files in the manifest.
func (repo *Repo) GetFiles(args FilesArgs) ([]*ManifestFile, error) {
	revlog, err := repo.readrevlog("store/00manifest.i")
	if err != nil {
		return nil, err
	}
	want := revlog.Len() - 1
	if args.Revision != "" {
		r, err := repo.parserevnum(args.Revision)
		if err != nil {
			return nil, err
		}
		if len(r) > 0 {
			// notyet
			want = r[0]
		}
	}
	wanted := args.Filenames
	files, err := parsemanifest(revlog, want)
	if wanted != nil {
		var newfiles []*ManifestFile
		for _, want := range wanted {
			if want == "" {
				newfiles = append(newfiles, files...)
				continue
			}
			start := sort.Search(len(files), func(i int) bool {
				return files[i].Name >= want
			})
			for i := start; i < len(files); i++ {
				file := files[i]
				if file.Name == want {
					newfiles = append(newfiles, file)
					continue
				}
				if len(file.Name) <= len(want) {
					break
				}
				if want[len(want)-1] == '/' && file.Name[0:len(want)] == want {
					newfiles = append(newfiles, file)
					continue
				}
				if file.Name[len(want)] == '/' && file.Name[0:len(want)] == want {
					newfiles = append(newfiles, file)
					continue
				}
				break
			}
		}
		files = newfiles
	}
	return files, err
}

type FileDataArgs struct {
	Filename string
	Revision string
}

// Get the data for a file.
func (repo *Repo) GetFileData(args FileDataArgs) ([]byte, error) {
	revlog, err := repo.readchangelog()
	if err != nil {
		return nil, err
	}
	want := revlog.Len() - 1
	if args.Revision != "" {
		r, err := repo.parserevnum(args.Revision)
		if err != nil {
			return nil, err
		}
		if len(r) != 1 {
			return nil, fmt.Errorf("only one revision")
		}
		want = r[0]
	}
	filename := args.Filename
	err = repo.checkfilename(filename)
	if err != nil {
		return nil, err
	}
	revlog, err = repo.readrevlog(storename(filename))
	if err != nil {
		return nil, err
	}
	var prev *Rev
	for i := 0; i < revlog.Len(); i++ {
		rev, _ := revlog.Get(i)
		if rev.Linkrev > want {
			break
		}
		prev = rev
	}
	if prev == nil {
		return nil, fmt.Errorf("file not found in revision")
	}
	idx := revlog.FindLink(prev.Linkrev)
	base, _, err := revlog.GetData(idx)
	return base, err
}
