package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/decko/wiki-go/internal/config"
	"github.com/decko/wiki-go/internal/i18n"
	"github.com/decko/wiki-go/internal/tags"
	"github.com/decko/wiki-go/internal/types"
	"github.com/decko/wiki-go/internal/utils"
)

type graphNode struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Type  string   `json:"type"`
	Tags  []string `json:"tags"`
}

type graphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type graphData struct {
	Nodes []graphNode `json:"nodes"`
	Links []graphLink `json:"links"`
}

// GraphDataHandler returns JSON graph data with nodes (documents) and
// links (shared tags >= 2).
func GraphDataHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config, tagIdx *tags.TagIndex) {
	docs := tagIdx.GetAllDocs()

	nodes := make([]graphNode, 0, len(docs))
	for _, d := range docs {
		nodes = append(nodes, graphNode{
			ID:    d.Path,
			Title: d.Title,
			Type:  d.Type,
			Tags:  d.Tags,
		})
	}

	// Build links from shared tags: two documents with >=2 common tags get an edge.
	var links []graphLink
	for i := 0; i < len(docs); i++ {
		if len(docs[i].Tags) < 2 {
			continue
		}
		setI := make(map[string]bool, len(docs[i].Tags))
		for _, t := range docs[i].Tags {
			setI[t] = true
		}
		for j := i + 1; j < len(docs); j++ {
			shared := 0
			for _, t := range docs[j].Tags {
				if setI[t] {
					shared++
				}
			}
			if shared >= 2 {
				links = append(links, graphLink{
					Source: docs[i].Path,
					Target: docs[j].Path,
					Type:   "tag",
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graphData{Nodes: nodes, Links: links})
}

// GraphPageHandler renders a page with a D3.js force-directed graph.
func GraphPageHandler(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	nav, _ := utils.BuildNavigation(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)

	graphHTML := `
<h1>Knowledge Graph</h1>
<div id="graph-container" style="width:100%;height:600px;border:1px solid var(--border-color,#ddd);border-radius:8px;overflow:hidden;"></div>
<script src="https://d3js.org/d3.v7.min.js"></script>
<script>
(function(){
  var container = document.getElementById('graph-container');
  var width = container.clientWidth;
  var height = container.clientHeight || 600;

  var svg = d3.select('#graph-container').append('svg')
    .attr('width', width).attr('height', height);

  var g = svg.append('g');
  svg.call(d3.zoom().on('zoom', function(e){ g.attr('transform', e.transform); }));

  fetch('/api/graph').then(function(r){ return r.json(); }).then(function(data){
    if(!data.nodes || data.nodes.length === 0){
      var msg = document.createElement('p');
      msg.style.cssText = 'text-align:center;padding:40px;opacity:0.6;';
      msg.textContent = 'No documents with tags found.';
      container.replaceChildren(msg);
      return;
    }

    var simulation = d3.forceSimulation(data.nodes)
      .force('link', d3.forceLink(data.links).id(function(d){return d.id;}).distance(120))
      .force('charge', d3.forceManyBody().strength(-200))
      .force('center', d3.forceCenter(width/2, height/2))
      .force('collision', d3.forceCollide().radius(30));

    var link = g.append('g')
      .selectAll('line')
      .data(data.links)
      .join('line')
      .attr('stroke', '#999')
      .attr('stroke-opacity', 0.4)
      .attr('stroke-width', 1.5);

    var node = g.append('g')
      .selectAll('g')
      .data(data.nodes)
      .join('g')
      .call(d3.drag()
        .on('start', function(e,d){ if(!e.active) simulation.alphaTarget(0.3).restart(); d.fx=d.x; d.fy=d.y; })
        .on('drag', function(e,d){ d.fx=e.x; d.fy=e.y; })
        .on('end', function(e,d){ if(!e.active) simulation.alphaTarget(0); d.fx=null; d.fy=null; })
      );

    node.append('circle')
      .attr('r', 8)
      .attr('fill', function(d){
        var colors = {'Transcript':'#e74c3c','Article':'#3498db','Repository':'#2ecc71','Paper':'#9b59b6','Bookmark':'#95a5a6','Issue':'#e67e22','Document':'#f39c12','Documentation':'#1abc9c','Ticket':'#e67e22','Tool':'#34495e'};
        return colors[d.type] || '#6c5ce7';
      })
      .attr('stroke', '#fff')
      .attr('stroke-width', 1.5);

    node.append('text')
      .text(function(d){ return d.title.length > 25 ? d.title.slice(0,25)+'...' : d.title; })
      .attr('x', 12)
      .attr('y', 4)
      .attr('font-size', '11px')
      .attr('fill', 'currentColor');

    node.on('click', function(e,d){ window.location.href = d.id; });
    node.style('cursor', 'pointer');

    node.append('title').text(function(d){ return d.title + (d.tags && d.tags.length ? '\nTags: '+d.tags.join(', ') : ''); });

    simulation.on('tick', function(){
      link.attr('x1',function(d){return d.source.x;}).attr('y1',function(d){return d.source.y;})
          .attr('x2',function(d){return d.target.x;}).attr('y2',function(d){return d.target.y;});
      node.attr('transform',function(d){return 'translate('+d.x+','+d.y+')';});
    });
  });
})();
</script>
`
	data := &types.PageData{
		Navigation: &types.NavTree{Root: nav, AlwaysOpen: cfg.Wiki.AlwaysOpenChildrenInSidebar},
		Content:    template.HTML(graphHTML),
		Breadcrumbs: []types.BreadcrumbItem{
			{Title: "Home", Path: "/"},
			{Title: "Knowledge Graph", Path: "/graph/", IsLast: true},
		},
		Config:             cfg,
		Title:              "Knowledge Graph",
		CurrentDir:         &types.NavItem{Title: "Knowledge Graph", Path: "/graph/"},
		AvailableLanguages: i18n.GetAvailableLanguages(),
	}
	renderTemplate(w, data)
}
