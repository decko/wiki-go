package handlers

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"

	"wiki-go/internal/config"
	"wiki-go/internal/i18n"
	"wiki-go/internal/tags"
	"wiki-go/internal/types"
	"wiki-go/internal/utils"
)

// TagsHandler handles tag listing and per-tag document listing.
//   GET /tags/          — all tags with counts
//   GET /tags/some-tag  — documents tagged "some-tag"
func TagsHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	tagName := strings.TrimPrefix(r.URL.Path, "/tags/")
	tagName = strings.TrimSuffix(tagName, "/")

	if tagName == "" {
		renderAllTags(w, r, cfg, tagIdx)
	} else {
		renderTagDocs(w, r, cfg, tagIdx, tagName)
	}
}

func renderAllTags(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	allTags := tagIdx.GetAllTags()

	var sb strings.Builder
	sb.WriteString(`<h1>Tags</h1>`)

	if len(allTags) == 0 {
		sb.WriteString(`<p>No tags found.</p>`)
	} else {
		sb.WriteString(`<div class="okf-tags" style="gap:8px;flex-wrap:wrap;display:flex;">`)
		for _, tc := range allTags {
			sb.WriteString(fmt.Sprintf(
				`<a href="/tags/%s" class="okf-tag" style="font-size:0.95em;padding:4px 12px;">%s <span style="opacity:0.6;">(%d)</span></a>`,
				html.EscapeString(tc.Tag),
				html.EscapeString(tc.Tag),
				tc.Count,
			))
		}
		sb.WriteString(`</div>`)
	}

	nav, _ := utils.BuildNavigation(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	data := &types.PageData{
		Navigation:         &types.NavTree{Root: nav, AlwaysOpen: cfg.Wiki.AlwaysOpenChildrenInSidebar},
		Content:            template.HTML(sb.String()),
		Breadcrumbs:        []types.BreadcrumbItem{{Title: "Home", Path: "/"}, {Title: "Tags", Path: "/tags/", IsLast: true}},
		Config:             cfg,
		Title:              "Tags",
		CurrentDir:         &types.NavItem{Title: "Tags", Path: "/tags/"},
		AvailableLanguages: i18n.GetAvailableLanguages(),
	}
	renderTemplate(w, data)
}

func renderTagDocs(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex, tag string) {
	docs := tagIdx.GetDocsByTag(tag)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<h1>Tag: %s</h1>`, html.EscapeString(tag)))

	if len(docs) == 0 {
		sb.WriteString(`<p>No documents found with this tag.</p>`)
	} else {
		sb.WriteString(fmt.Sprintf(`<p>%d document(s)</p>`, len(docs)))
		sb.WriteString(`<ul>`)
		for _, doc := range docs {
			sb.WriteString(`<li>`)
			if doc.Type != "" {
				sb.WriteString(fmt.Sprintf(`<span class="okf-type-badge" style="margin-right:6px;">%s</span> `, html.EscapeString(doc.Type)))
			}
			sb.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(doc.Path), html.EscapeString(doc.Title)))
			if doc.Description != "" {
				sb.WriteString(fmt.Sprintf(`<br><small style="opacity:0.7;">%s</small>`, html.EscapeString(doc.Description)))
			}
			sb.WriteString(`</li>`)
		}
		sb.WriteString(`</ul>`)
	}

	nav, _ := utils.BuildNavigation(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	data := &types.PageData{
		Navigation: &types.NavTree{Root: nav, AlwaysOpen: cfg.Wiki.AlwaysOpenChildrenInSidebar},
		Content:    template.HTML(sb.String()),
		Breadcrumbs: []types.BreadcrumbItem{
			{Title: "Home", Path: "/"},
			{Title: "Tags", Path: "/tags/"},
			{Title: tag, Path: "/tags/" + tag, IsLast: true},
		},
		Config:             cfg,
		Title:              "Tag: " + tag,
		CurrentDir:         &types.NavItem{Title: "Tag: " + tag, Path: "/tags/" + tag},
		AvailableLanguages: i18n.GetAvailableLanguages(),
	}
	renderTemplate(w, data)
}
