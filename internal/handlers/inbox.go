package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"

	"github.com/decko/wiki-go/internal/config"
	"github.com/decko/wiki-go/internal/i18n"
	"github.com/decko/wiki-go/internal/tags"
	"github.com/decko/wiki-go/internal/types"
	"github.com/decko/wiki-go/internal/utils"
)

func InboxHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	docs := tagIdx.GetInboxDocs()

	var sb strings.Builder
	sb.WriteString(`<h1>Inbox</h1>`)
	sb.WriteString(fmt.Sprintf(`<p>%d item(s) awaiting review</p>`, len(docs)))

	if len(docs) == 0 {
		sb.WriteString(`<p style="opacity:0.6;margin-top:32px;">All clear — nothing to review.</p>`)
	} else {
		for _, doc := range docs {
			sb.WriteString(`<div class="okf-inbox-item">`)

			// Header row: type badge + title + actions
			sb.WriteString(`<div class="okf-inbox-header">`)
			sb.WriteString(`<div>`)
			if doc.Type != "" {
				sb.WriteString(fmt.Sprintf(`<span class="okf-type-badge">%s</span> `, html.EscapeString(doc.Type)))
			}
			sb.WriteString(fmt.Sprintf(`<a href="%s" class="okf-inbox-title">%s</a>`, html.EscapeString(doc.Path), html.EscapeString(doc.Title)))
			sb.WriteString(`</div>`)
			sb.WriteString(`<div class="okf-inbox-actions">`)
			sb.WriteString(fmt.Sprintf(`<button class="okf-btn okf-btn-approve" onclick="inboxAction('approve','%s',this)" title="Approve">✓</button>`, html.EscapeString(doc.FilePath)))
			sb.WriteString(fmt.Sprintf(`<button class="okf-btn okf-btn-dismiss" onclick="inboxAction('dismiss','%s',this)" title="Dismiss">✕</button>`, html.EscapeString(doc.FilePath)))
			sb.WriteString(`</div>`)
			sb.WriteString(`</div>`)

			// Description
			if doc.Description != "" && doc.Description != "Bookmarked from Firefox tabs." {
				desc := doc.Description
				if len(desc) > 180 {
					desc = desc[:177] + "..."
				}
				sb.WriteString(fmt.Sprintf(`<p class="okf-inbox-desc">%s</p>`, html.EscapeString(desc)))
			}

			// Tags
			if len(doc.Tags) > 0 {
				sb.WriteString(`<div class="okf-inbox-tags">`)
				for _, tag := range doc.Tags {
					sb.WriteString(fmt.Sprintf(`<a href="/tags/%s" class="okf-inbox-tag">%s</a>`, html.EscapeString(tag), html.EscapeString(tag)))
				}
				sb.WriteString(`</div>`)
			}

			sb.WriteString(`</div>`)
		}
	}

	sb.WriteString(`
<script>
function inboxAction(action, filePath, btn) {
  if (action === 'dismiss' && !confirm('Delete this item permanently?')) return;
  var item = btn.closest('.okf-inbox-item');
  fetch('/api/inbox', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({action: action, file_path: filePath})
  }).then(function(r) { return r.json(); }).then(function(data) {
    if (data.success) {
      item.style.transition = 'opacity 0.3s';
      item.style.opacity = '0';
      setTimeout(function(){ item.remove(); }, 300);
      var count = document.querySelectorAll('.okf-inbox-item').length - 1;
      document.querySelector('h1 + p').textContent = count + ' item(s) awaiting review';
      if (count <= 0) location.reload();
    } else {
      alert('Error: ' + (data.error || 'unknown'));
    }
  });
}
</script>`)

	nav, _ := utils.BuildNavigation(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	data := &types.PageData{
		Navigation: &types.NavTree{Root: nav, AlwaysOpen: cfg.Wiki.AlwaysOpenChildrenInSidebar},
		Content:    template.HTML(sb.String()),
		Breadcrumbs: []types.BreadcrumbItem{
			{Title: "Home", Path: "/"},
			{Title: "Inbox", Path: "/inbox/", IsLast: true},
		},
		Config:             cfg,
		Title:              "Inbox",
		CurrentDir:         &types.NavItem{Title: "Inbox", Path: "/inbox/"},
		AvailableLanguages: i18n.GetAvailableLanguages(),
	}
	renderTemplate(w, data)
}

type inboxRequest struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
}

type inboxResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func InboxAPIHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req inboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inboxResponse{Error: "Invalid request"})
		return
	}

	var err error
	switch req.Action {
	case "approve":
		err = tagIdx.ApproveDoc(req.FilePath)
	case "dismiss":
		err = tagIdx.DismissDoc(req.FilePath)
	default:
		err = fmt.Errorf("unknown action: %s", req.Action)
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(inboxResponse{Error: err.Error()})
	} else {
		json.NewEncoder(w).Encode(inboxResponse{Success: true})
	}
}
