package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"wiki-go/internal/frontmatter"
)

// DocumentRef represents a document with OKF metadata.
type DocumentRef struct {
	Path        string
	Title       string
	Description string
	Type        string
	Tags        []string
	Status      string
	FilePath    string
}

// TagCount holds a tag name and the number of documents it appears in.
type TagCount struct {
	Tag   string
	Count int
}

// TagIndex maintains an inverted index from tags to documents.
type TagIndex struct {
	mu      sync.RWMutex
	tags    map[string][]DocumentRef // tag -> documents
	docTags map[string][]string     // docPath -> tags
	allDocs []DocumentRef           // all documents with frontmatter
	rootDir string
	docsDir string
	modTime time.Time
}

// NewTagIndex creates a new TagIndex for the given root and documents directory.
func NewTagIndex(rootDir, docsDir string) *TagIndex {
	return &TagIndex{
		rootDir: rootDir,
		docsDir: docsDir,
		tags:    make(map[string][]DocumentRef),
		docTags: make(map[string][]string),
	}
}

// Build walks all .md files and builds the tag index.
func (idx *TagIndex) Build() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.tags = make(map[string][]DocumentRef)
	idx.docTags = make(map[string][]string)
	idx.allDocs = nil

	docsRoot := filepath.Join(idx.rootDir, idx.docsDir)
	resolvedRoot, err := filepath.EvalSymlinks(docsRoot)
	if err != nil {
		resolvedRoot = docsRoot
	}

	mdFiles := collectMDFiles(docsRoot)

	for _, path := range mdFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		_, rawFields, _, hasFM := frontmatter.ParseWithRawFields(string(content))
		if !hasFM || rawFields == nil {
			continue
		}

		// Compute the URL path: try rel against both original and resolved root
		relPath, relErr := filepath.Rel(docsRoot, path)
		if relErr != nil {
			relPath, relErr = filepath.Rel(resolvedRoot, path)
			if relErr != nil {
				continue
			}
		}
		relPath = filepath.ToSlash(relPath)

		var urlPath string
		if strings.HasSuffix(relPath, "/document.md") {
			urlPath = "/" + strings.TrimSuffix(relPath, "/document.md")
		} else if relPath == "document.md" {
			urlPath = "/"
		} else {
			urlPath = "/" + strings.TrimSuffix(relPath, ".md")
		}

		doc := DocumentRef{
			Path: urlPath,
		}

		if v, ok := rawFields["title"]; ok {
			doc.Title = fmt.Sprintf("%v", v)
		}
		if doc.Title == "" {
			doc.Title = extractFirstHeading(string(content))
		}
		if doc.Title == "" {
			doc.Title = filepath.Base(urlPath)
		}

		if v, ok := rawFields["description"]; ok {
			doc.Description = fmt.Sprintf("%v", v)
		}

		if v, ok := rawFields["type"]; ok {
			doc.Type = fmt.Sprintf("%v", v)
		}

		doc.Tags = extractTags(rawFields)
		doc.FilePath = path

		if v, ok := rawFields["status"]; ok {
			doc.Status = fmt.Sprintf("%v", v)
		}

		idx.allDocs = append(idx.allDocs, doc)

		if len(doc.Tags) > 0 {
			idx.docTags[doc.Path] = doc.Tags
			for _, tag := range doc.Tags {
				idx.tags[tag] = append(idx.tags[tag], doc)
			}
		}
	}

	// Record the mod time of the docs directory
	if fi, err := os.Stat(docsRoot); err == nil {
		idx.modTime = fi.ModTime()
	}
}

// rebuildIfStale checks the docs directory mtime and rebuilds if changed.
func (idx *TagIndex) rebuildIfStale() {
	docsRoot := filepath.Join(idx.rootDir, idx.docsDir)
	fi, err := os.Stat(docsRoot)
	if err != nil {
		return
	}

	idx.mu.RLock()
	stale := idx.modTime.IsZero() || fi.ModTime().After(idx.modTime)
	idx.mu.RUnlock()

	if stale {
		idx.Build()
	}
}

// GetTags returns the full tag-to-documents map.
func (idx *TagIndex) GetTags() map[string][]DocumentRef {
	idx.rebuildIfStale()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Return a copy
	result := make(map[string][]DocumentRef, len(idx.tags))
	for k, v := range idx.tags {
		result[k] = v
	}
	return result
}

// GetDocsByTag returns all documents tagged with the given tag.
func (idx *TagIndex) GetDocsByTag(tag string) []DocumentRef {
	idx.rebuildIfStale()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.tags[tag]
}

// GetAllDocs returns all indexed documents.
func (idx *TagIndex) GetAllDocs() []DocumentRef {
	idx.rebuildIfStale()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.allDocs
}

// GetAllTags returns all tags sorted by count descending.
func (idx *TagIndex) GetAllTags() []TagCount {
	idx.rebuildIfStale()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	counts := make([]TagCount, 0, len(idx.tags))
	for tag, docs := range idx.tags {
		counts = append(counts, TagCount{Tag: tag, Count: len(docs)})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}
		return counts[i].Tag < counts[j].Tag
	})
	return counts
}

// GetInboxDocs returns all documents with status "inbox".
func (idx *TagIndex) GetInboxDocs() []DocumentRef {
	idx.rebuildIfStale()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []DocumentRef
	for _, doc := range idx.allDocs {
		if doc.Status == "inbox" {
			result = append(result, doc)
		}
	}
	return result
}

// ApproveDoc removes the "status: inbox" line from a document's frontmatter.
func (idx *TagIndex) ApproveDoc(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	s := string(content)
	// Remove status: inbox line from frontmatter
	s = strings.Replace(s, "status: inbox\n", "", 1)

	if err := os.WriteFile(filePath, []byte(s), 0644); err != nil {
		return err
	}

	// Force rebuild on next access
	idx.mu.Lock()
	idx.modTime = time.Time{}
	idx.mu.Unlock()

	return nil
}

// DismissDoc removes a document file from disk.
func (idx *TagIndex) DismissDoc(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return err
	}

	idx.mu.Lock()
	idx.modTime = time.Time{}
	idx.mu.Unlock()

	return nil
}

// extractTags pulls the tags field from raw frontmatter fields.
func extractTags(rawFields map[string]interface{}) []string {
	v, ok := rawFields["tags"]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []interface{}:
		tags := make([]string, 0, len(t))
		for _, item := range t {
			tags = append(tags, fmt.Sprintf("%v", item))
		}
		return tags
	case string:
		// Comma-separated string
		parts := strings.Split(t, ",")
		tags := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				tags = append(tags, p)
			}
		}
		return tags
	}
	return nil
}

// collectMDFiles walks a directory tree following symlinks, returning all .md file paths.
func collectMDFiles(root string) []string {
	var files []string
	collectMDInner(root, &files)
	return files
}

func collectMDInner(dir string, files *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Stat(path) // follows symlinks
		if err != nil {
			continue
		}
		if info.IsDir() {
			collectMDInner(path, files)
		} else if strings.HasSuffix(entry.Name(), ".md") {
			*files = append(*files, path)
		}
	}
}

// extractFirstHeading returns the first # heading from markdown content.
func extractFirstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
