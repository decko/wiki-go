package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wiki-go/internal/auth"
	"wiki-go/internal/comments"
	"wiki-go/internal/config"
	"wiki-go/internal/frontmatter"
	"wiki-go/internal/i18n"
	"wiki-go/internal/tags"
	"wiki-go/internal/types"
	"wiki-go/internal/utils"
)

// PageHandler handles requests for pages
func PageHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	// Add cache control headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Detect edit mode from query parameter
	mode := r.URL.Query().Get("mode")
	isEditMode := mode == "edit"

	// Edit mode requires authentication and editor/admin role
	if isEditMode {
		session := auth.GetSession(r)
		if session == nil {
			// User not authenticated - redirect to view mode
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
		// Check if user has editor or admin role
		if !auth.RequireRole(r, "editor") {
			// User lacks required role - redirect to view mode
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
	}

	// Get the requested path
	path := r.URL.Path
	if path == "/" {
		HomeHandler(w, r, cfg)
		return
	}

	// Clean and decode the path
	path = filepath.Clean(path)
	path = strings.TrimSuffix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Check access
	session := auth.GetSession(r)
	if !auth.CanAccessDocument(path, session, cfg) {
		if session == nil {
			http.Redirect(w, r, "/login?redirect="+url.QueryEscape(path), http.StatusFound)
			return
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Build navigation
	nav, err := utils.BuildNavigation(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter navigation based on access
	nav = utils.FilterNavigation(nav, func(p string) bool {
		return auth.CanAccessDocument(p, session, cfg)
	})

	// Mark active navigation item
	utils.MarkActiveNavItem(nav, path)

	// Get the full filesystem path - adjust to use documents subdirectory
	fsPath := filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir, decodedPath)

	// Determine whether this is a directory, a flat .md file, or not found
	isFlatFile := false
	flatFilePath := ""
	info, err := os.Stat(fsPath)
	if err != nil || !info.IsDir() {
		// Not a directory — try fsPath + ".md" as a flat file
		candidate := fsPath + ".md"
		if fi, ferr := os.Stat(candidate); ferr == nil && !fi.IsDir() {
			isFlatFile = true
			flatFilePath = candidate
			info = fi
		} else {
			NotFoundHandler(w, r, cfg)
			return
		}
	}

	// Find the navigation item for breadcrumbs
	navItem := utils.FindNavItem(nav, path)
	if navItem == nil {
		navItem = &types.NavItem{
			Title: utils.FormatDirName(filepath.Base(decodedPath)),
			Path:  path,
			IsDir: !isFlatFile,
		}
	}

	// Generate breadcrumbs
	breadcrumbs := generateBreadcrumbs(nav, path)

	var content template.HTML
	var lastModified time.Time
	var dirContent template.HTML
	var rawContent string // Raw markdown content for edit mode
	var frontmatterData map[string]interface{}

	var docPath string
	var docInfo os.FileInfo

	if isFlatFile {
		// Flat .md file — read it directly
		docPath = flatFilePath
		docInfo = info
	} else {
		// Directory — look for document.md, fall back to index.md
		docPath = filepath.Join(fsPath, "document.md")
		docInfo, err = os.Stat(docPath)
		if err != nil {
			docPath = filepath.Join(fsPath, "index.md")
			docInfo, err = os.Stat(docPath)
			if err != nil {
				docInfo = nil
			}
		}
	}

	if docInfo != nil {
		// Read and render the markdown file
		mdContent, err := os.ReadFile(docPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If in edit mode, store raw content with frontmatter preserved
		if isEditMode {
			rawContent = string(mdContent)
		}

		// Parse frontmatter with raw fields for OKF metadata
		metadata, rawFields, _, hasFrontmatter := frontmatter.ParseWithRawFields(string(mdContent))
		documentLayout := ""
		if hasFrontmatter {
			documentLayout = metadata.Layout
			frontmatterData = rawFields
		}

		// Use the document path for rendering to handle local file references
		content = template.HTML(utils.RenderMarkdownWithPath(string(mdContent), decodedPath))

		// If content is empty but document exists, ensure we have something truthy for template conditions
		if strings.TrimSpace(string(content)) == "" {
			content = template.HTML(" ") // Single space to make it truthy but effectively empty
		}

		lastModified = docInfo.ModTime()

		// Update the document layout in the page data
		navItem.DocumentLayout = documentLayout
	}

	// Directory listing only applies when fsPath is a directory
	if !isFlatFile {
		// List directory contents
		files, err := os.ReadDir(fsPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Build directory listing HTML with metadata from tag index
		allDocs := tagIdx.GetAllDocs()
		docByPath := make(map[string]tags.DocumentRef, len(allDocs))
		for _, d := range allDocs {
			docByPath[d.Path] = d
		}

		var dirItems []string
		for _, f := range files {
			name := f.Name()
			if strings.HasPrefix(name, ".") || name == "document.md" || name == "index.md" || name == "log.md" {
				continue
			}

			entryPath := filepath.Join(fsPath, name)
			info, statErr := os.Stat(entryPath)
			if statErr != nil {
				continue
			}

			if info.IsDir() {
				urlPath := filepath.Join(path, name)
				urlPath = strings.ReplaceAll(urlPath, "\\", "/")

				if !auth.CanAccessDocument(urlPath, session, cfg) {
					continue
				}

				dirTitle := utils.FormatDirName(name)
				subDocPath := filepath.Join(entryPath, "document.md")
				if _, err := os.Stat(subDocPath); err == nil {
					dirTitle = utils.GetDocumentTitle(entryPath)
				}
				dirItems = append(dirItems, fmt.Sprintf(`<div class="directory-item is-dir"><a href="%s">%s</a></div>`,
					urlPath, dirTitle))
			} else if strings.HasSuffix(name, ".md") {
				slug := strings.TrimSuffix(name, ".md")
				urlPath := filepath.Join(path, slug)
				urlPath = strings.ReplaceAll(urlPath, "\\", "/")

				var item strings.Builder
				item.WriteString(`<div class="okf-dir-item">`)

				if doc, ok := docByPath[urlPath]; ok {
					if doc.Type != "" {
						item.WriteString(fmt.Sprintf(`<span class="okf-type-badge">%s</span> `, doc.Type))
					}
					item.WriteString(fmt.Sprintf(`<a href="%s"><strong>%s</strong></a>`, urlPath, doc.Title))
					if doc.Description != "" {
						desc := doc.Description
						if len(desc) > 150 {
							desc = desc[:147] + "..."
						}
						item.WriteString(fmt.Sprintf(`<div class="okf-dir-desc">%s</div>`, desc))
					}
					if len(doc.Tags) > 0 {
						item.WriteString(`<div class="okf-tags" style="margin-top:4px;">`)
						for _, tag := range doc.Tags {
							item.WriteString(fmt.Sprintf(`<a href="/tags/%s" class="okf-tag">%s</a>`, tag, tag))
						}
						item.WriteString(`</div>`)
					}
				} else {
					title := utils.GetFlatMDTitle(entryPath)
					if title == "" {
						title = utils.FormatDirName(slug)
					}
					item.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, urlPath, title))
				}

				item.WriteString(`</div>`)
				dirItems = append(dirItems, item.String())
			}
		}

		if len(dirItems) > 0 {
			dirContent = template.HTML(strings.Join(dirItems, "\n"))
		}
	}

	// If no document content exists, show directory title and listing
	if docInfo == nil {
		content = template.HTML(fmt.Sprintf("<h1>%s</h1>", navItem.Title))
		lastModified = info.ModTime()
	}

	// Check if this is a document (not a directory) by checking if there's content and no trailing slash
	isDocument := docInfo != nil && content != "" && !strings.HasSuffix(r.URL.Path, "/")

	// Get authentication status for ALL pages
	var commentsList []comments.Comment
	var commentsAllowed bool = false // Default to false
	var isAuthenticated bool

	// Get authentication status - do this for ALL pages
	// session is already retrieved at the beginning of the function
	isAuthenticated = session != nil

	// Get user role
	userRole := ""
	if isAuthenticated && session != nil {
		userRole = session.Role
	}

	// Comments are only available for documents
	if isDocument {
		// UNCONDITIONALLY check system-wide setting first
		if cfg.Wiki.DisableComments {
			// If comments are disabled system-wide, force commentsAllowed to false
			commentsAllowed = false
		} else {
			// Only check document-specific settings if system allows comments
			mdContent, _ := os.ReadFile(docPath)
			commentsAllowed = comments.AreCommentsAllowed(string(mdContent))

			// Only load comments if they're allowed
			if commentsAllowed {
				commentsList, _ = comments.GetComments(decodedPath)

				// Process comments (render markdown, format timestamps)
				for i := range commentsList {
					// Use template.HTML to properly render the HTML without escaping
					commentsList[i].RenderedHTML = template.HTML(utils.RenderMarkdown(commentsList[i].Content))
					commentsList[i].FormattedTime = comments.FormatCommentTime(commentsList[i].Timestamp)
				}
			}
		}
	}

	// Prepare template data
	data := &types.PageData{
		Navigation:         &types.NavTree{Root: nav, AlwaysOpen: cfg.Wiki.AlwaysOpenChildrenInSidebar},
		Content:            content,
		DirContent:         dirContent,
		Breadcrumbs:        breadcrumbs,
		Config:             cfg,
		LastModified:       lastModified,
		CurrentDir:         navItem,
		AvailableLanguages: i18n.GetAvailableLanguages(),
		Comments:           commentsList,
		CommentsAllowed:    commentsAllowed,
		IsAuthenticated:    isAuthenticated,
		UserRole:           userRole,
		DocPath:            decodedPath,
		DocumentLayout:     navItem.DocumentLayout,
		IsEditMode:         isEditMode,
		RawContent:         rawContent,         // Pass raw markdown content for edit mode
		FrontmatterData:    frontmatterData,    // OKF frontmatter metadata
	}

	renderTemplate(w, data)
}

// generateBreadcrumbs creates a breadcrumb trail from a path
func generateBreadcrumbs(nav *types.NavItem, path string) []types.BreadcrumbItem {
	if path == "" || path == "/" {
		return []types.BreadcrumbItem{{Title: "Home", Path: "/", IsLast: true}}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	breadcrumbs := make([]types.BreadcrumbItem, 0, len(parts)+1)

	// Always start with Home
	breadcrumbs = append(breadcrumbs, types.BreadcrumbItem{
		Title:  "Home",
		Path:   "/",
		IsLast: false,
	})

	currentPath := ""
	for i, part := range parts {
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}

		item := utils.FindNavItem(nav, "/"+currentPath)
		if item != nil {
			breadcrumbs = append(breadcrumbs, types.BreadcrumbItem{
				Title:  item.Title,
				Path:   "/" + currentPath,
				IsLast: i == len(parts)-1,
			})
		}
	}

	return breadcrumbs
}
