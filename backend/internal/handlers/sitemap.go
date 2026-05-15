package handlers

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/Ankitsinghchadda/InterviewPrep/internal/repository"
)

// SitemapHandler emits sitemap.xml for SEO crawlers. Routed at the LB level
// to /sitemap.xml so search engines find it at the well-known location.
type SitemapHandler struct {
	Repo       *repository.QuestionRepo
	Categories *repository.CategoryRepo
	// BaseURL is the canonical public origin, e.g. "https://10xinterview.com".
	// Wired from config.FrontendURL at startup.
	BaseURL string
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (h *SitemapHandler) Serve(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(h.BaseURL, "/")
	if base == "" {
		base = "https://" + r.Host
	}

	// Static landing pages first so they're always present even if the DB is
	// empty. Mirrors what the old static sitemap.xml had.
	urls := []sitemapURL{
		{Loc: base + "/", ChangeFreq: "weekly", Priority: "1.0"},
		{Loc: base + "/contact", ChangeFreq: "monthly", Priority: "0.5"},
		{Loc: base + "/login", ChangeFreq: "yearly", Priority: "0.3"},
	}

	// Topic landing pages (/topics/:slug). These are the highest-value SEO
	// surface — each targets a keyword like "javascript interview questions".
	if h.Categories != nil {
		if cats, err := h.Categories.List(r.Context(), ""); err == nil {
			for _, c := range cats {
				urls = append(urls, sitemapURL{
					Loc:        base + "/topics/" + c.Slug,
					ChangeFreq: "weekly",
					Priority:   "0.9",
				})
			}
		}
	}

	// Individual question pages.
	entries, err := h.Repo.ListPublicForSitemap(r.Context(), 50000)
	if err == nil {
		for _, e := range entries {
			urls = append(urls, sitemapURL{
				Loc:        base + "/questions/" + e.Slug,
				LastMod:    e.UpdatedAt.UTC().Format(time.RFC3339),
				ChangeFreq: "monthly",
				Priority:   "0.8",
			})
		}
	}
	// On a DB error we still serve the static URLs above so the file is never
	// completely empty — Google deindexes empty sitemaps.

	body, err := xml.MarshalIndent(urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}
