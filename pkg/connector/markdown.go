package connector

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// matrixHTMLToGFM converts Matrix message HTML to GitHub-flavored markdown.
func matrixHTMLToGFM(html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return htmlTagRe.ReplaceAllString(html, "")
	}
	// Inline formatting replacements on a clone of the HTML tree.
	doc.Find("strong, b").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("**" + s.Text() + "**")
	})
	doc.Find("em, i").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("*" + s.Text() + "*")
	})
	doc.Find("code").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("`" + s.Text() + "`")
	})
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if ok && href != "" {
			s.ReplaceWithHtml("[" + s.Text() + "](" + href + ")")
		}
	})
	doc.Find("br").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("\n")
	})
	var paragraphs []string
	doc.Find("p").Each(func(_ int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			paragraphs = append(paragraphs, text)
		}
	})
	if len(paragraphs) > 0 {
		return strings.Join(paragraphs, "\n\n")
	}
	return strings.TrimSpace(doc.Text())
}

// gfmToMatrixHTML converts GitHub markdown body to Matrix HTML (plain fallback).
func gfmToMatrixHTML(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	escaped := strings.ReplaceAll(body, "&", "&amp;")
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	return "<p>" + strings.ReplaceAll(escaped, "\n", "<br>") + "</p>"
}
