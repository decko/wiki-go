package utils

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"wiki-go/internal/goldext"
	"wiki-go/internal/types"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// NavItem represents a navigation item (directory)
type NavItem struct {
	Title    string
	Path     string
	IsDir    bool
	Children []*NavItem
	IsActive bool
}

// GetDocumentTitle extracts the first H1 title from document.md
func GetDocumentTitle(dirPath string) string {
	docPath := filepath.Join(dirPath, "document.md")
	file, err := os.Open(docPath)
	if err != nil {
		// If no document.md or can't read it, use directory name
		return FormatDirName(filepath.Base(dirPath))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			// Process emojis in the title
			title = goldext.EmojiPreprocessor(title, "")
			return title
		}
	}

	// If no H1 found, use directory name
	return FormatDirName(filepath.Base(dirPath))
}

// FormatDirName formats a directory name by replacing dashes with spaces and title casing
func FormatDirName(name string) string {
	// Replace dashes with spaces
	name = strings.ReplaceAll(name, "-", " ")

	// Title case the words using cases package
	titleCaser := cases.Title(language.English)
	return titleCaser.String(name)
}

// ToURLPath converts a filesystem path to a URL path
func ToURLPath(path string) string {
	// Convert spaces to dashes
	return strings.ReplaceAll(path, " ", "-")
}

// BuildNavigation builds the navigation structure from the root directory
func BuildNavigation(rootDir string, documentsDir string) (*types.NavItem, error) {
	root := &types.NavItem{
		Title:    "Wiki-Go",
		Path:     "/",
		IsDir:    true,
		Children: make([]*types.NavItem, 0),
	}

	// Create the documents directory path
	docsPath := filepath.Join(rootDir, documentsDir)

	// Check if documents directory exists
	if _, err := os.Stat(docsPath); os.IsNotExist(err) {
		// Create documents directory if it doesn't exist
		if err := os.MkdirAll(docsPath, 0755); err != nil {
			return nil, err
		}
	}

	err := walkDirFollowSymlinks(docsPath, func(path string, info os.FileInfo) {
		// Skip the documents directory itself
		if path == docsPath {
			return
		}

		base := filepath.Base(path)

		// Skip hidden entries and document.md
		if strings.HasPrefix(base, ".") || base == "document.md" {
			return
		}

		// Skip pages/home
		if path == filepath.Join(rootDir, "pages", "home") || path == filepath.Join(rootDir, "pages") {
			return
		}

		isDir := info.IsDir()
		isFlatMD := !isDir && strings.HasSuffix(base, ".md") && base != "index.md" && base != "log.md"

		if !isDir && !isFlatMD {
			return
		}

		// Create relative path for the URL
		relPath := strings.TrimPrefix(path, docsPath)
		relPath = strings.TrimPrefix(relPath, string(os.PathSeparator))
		relPath = filepath.ToSlash(relPath)

		var title string
		var urlPath string

		if isDir {
			title = GetDocumentTitle(path)
			parts := strings.Split(relPath, "/")
			current := root
			for i := 0; i < len(parts); i++ {
				urlPath = "/" + ToURLPath(filepath.ToSlash(filepath.Join(parts[:i+1]...)))
				var found *types.NavItem
				for _, child := range current.Children {
					if child.Path == urlPath {
						found = child
						break
					}
				}
				if found == nil {
					dirTitle := ""
					if i == len(parts)-1 {
						dirTitle = title
					} else {
						dirTitle = FormatDirName(parts[i])
					}
					found = &types.NavItem{
						Title:    dirTitle,
						Path:     urlPath,
						IsDir:    true,
						Children: make([]*types.NavItem, 0),
					}
					current.Children = append(current.Children, found)
				}
				current = found
			}
		} else {
			// Flat .md file — add as a leaf nav item
			slug := strings.TrimSuffix(base, ".md")
			urlPath = "/" + strings.TrimSuffix(relPath, ".md")
			title = GetFlatMDTitle(path)
			if title == "" {
				title = FormatDirName(slug)
			}

			// Find parent node
			parentRel := filepath.ToSlash(filepath.Dir(relPath))
			current := root
			if parentRel != "." {
				parts := strings.Split(parentRel, "/")
				for i := 0; i < len(parts); i++ {
					parentPath := "/" + ToURLPath(filepath.ToSlash(filepath.Join(parts[:i+1]...)))
					var found *types.NavItem
					for _, child := range current.Children {
						if child.Path == parentPath {
							found = child
							break
						}
					}
					if found == nil {
						found = &types.NavItem{
							Title:    FormatDirName(parts[i]),
							Path:     parentPath,
							IsDir:    true,
							Children: make([]*types.NavItem, 0),
						}
						current.Children = append(current.Children, found)
					}
					current = found
				}
			}

			// Check if already added
			exists := false
			for _, child := range current.Children {
				if child.Path == urlPath {
					exists = true
					break
				}
			}
			if !exists {
				current.Children = append(current.Children, &types.NavItem{
					Title:    title,
					Path:     urlPath,
					IsDir:    false,
					Children: make([]*types.NavItem, 0),
				})
			}
		}
	})

	return root, err
}

// walkDirFollowSymlinks walks a directory tree following symlinks.
func walkDirFollowSymlinks(root string, fn func(string, os.FileInfo)) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Stat(path) // follows symlinks
		if err != nil {
			continue
		}
		fn(path, info)
		if info.IsDir() {
			walkDirFollowSymlinks(path, fn)
		}
	}
	return nil
}

// GetFlatMDTitle reads the title from a flat .md file (frontmatter title: or first # heading).
func GetFlatMDTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			if strings.HasPrefix(line, "title:") {
				t := strings.TrimPrefix(line, "title:")
				t = strings.TrimSpace(t)
				t = strings.Trim(t, "\"'")
				return t
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// FindNavItem finds a navigation item by its path
func FindNavItem(root *types.NavItem, path string) *types.NavItem {
	if root == nil {
		return nil
	}

	// Clean up the path
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}

	if root.Path == path {
		return root
	}

	for _, child := range root.Children {
		if found := FindNavItem(child, path); found != nil {
			return found
		}
	}

	return nil
}

// MarkActiveNavItem marks the active navigation item and its parents
func MarkActiveNavItem(root *types.NavItem, currentPath string) {
	if root == nil {
		return
	}

	// Clean up the path
	currentPath = strings.TrimSuffix(currentPath, "/")
	if currentPath == "" {
		currentPath = "/"
	}

	// Mark this item if it matches
	if root.Path == currentPath {
		root.IsActive = true
	}

	// Mark this item if any child is active
	for _, child := range root.Children {
		MarkActiveNavItem(child, currentPath)
		if child.IsActive {
			root.IsActive = true
		}
	}
}

// FilterNavigation filters the navigation tree based on a predicate function
func FilterNavigation(node *types.NavItem, allow func(path string) bool) *types.NavItem {
	if node == nil {
		return nil
	}

	// Create a new node to avoid modifying the original tree
	newNode := &types.NavItem{
		Title:          node.Title,
		Path:           node.Path,
		IsDir:          node.IsDir,
		IsActive:       node.IsActive,
		DocumentLayout: node.DocumentLayout,
		Children:       make([]*types.NavItem, 0),
	}

	for _, child := range node.Children {
		// Check if child path is allowed
		if allow(child.Path) {
			// Recursively filter children
			filteredChild := FilterNavigation(child, allow)
			if filteredChild != nil {
				newNode.Children = append(newNode.Children, filteredChild)
			}
		}
	}

	return newNode
}
