package goldext

import (
	"github.com/decko/wiki-go/internal/frontmatter"
)

// FrontmatterPreprocessor removes frontmatter from markdown content before rendering
func FrontmatterPreprocessor(markdown string, _ string) string {
	// Check if content has frontmatter
	if !frontmatter.HasFrontmatter(markdown) {
		return markdown
	}

	// Parse frontmatter and get content without it
	_, contentWithoutFrontmatter, _ := frontmatter.Parse(markdown)
	return contentWithoutFrontmatter
}