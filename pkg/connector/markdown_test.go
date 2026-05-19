package connector

import "testing"

func TestMatrixHTMLToGFM(t *testing.T) {
	md := matrixHTMLToGFM(`<p>Hello <strong>world</strong></p>`)
	if md != "Hello **world**" {
		t.Fatalf("unexpected markdown: %q", md)
	}
}

func TestGFMToMatrixHTML(t *testing.T) {
	html := gfmToMatrixHTML("line one\nline two")
	if html == "" {
		t.Fatal("expected html output")
	}
	if html != "<p>line one<br>line two</p>" {
		t.Fatalf("unexpected html: %q", html)
	}
}
