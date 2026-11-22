package search

import (
	"context"
	"testing"
)

// The index must list the pages it holds, ordered by their address.
func TestPagesListsEveryPage(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	pages, err := idx.Pages(context.Background(), "")
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	if len(pages) != len(sampleDocs()) {
		t.Fatalf("got %d pages, want %d: %+v", len(pages), len(sampleDocs()), pages)
	}
	if pages[0].URL != "/adr/adr-001/" {
		t.Fatalf("first page = %q, want the lowest address", pages[0].URL)
	}
	if pages[0].Title != "ADR-001: Staging" {
		t.Fatalf("title = %q", pages[0].Title)
	}
}

// A prefix keeps only the pages whose address starts with it.
func TestPagesKeepsOnlyThePrefixedPages(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), sampleDocs()); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	pages, err := idx.Pages(context.Background(), "/deploy")
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1: %+v", len(pages), pages)
	}
	if pages[0].URL != "/deploy/kubernetes/" {
		t.Fatalf("page = %+v", pages[0])
	}
}

// The prefix is compared as it is written: a % in it is a part of an
// address, not a wildcard.
func TestPagesReadsThePrefixAsItIsWritten(t *testing.T) {
	idx := openTestIndex(t)
	if err := idx.Upsert(context.Background(), []Document{
		{URL: "/a%b/", Title: "A percent"},
		{URL: "/other/", Title: "Other"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	pages, err := idx.Pages(context.Background(), "/a%b/")
	if err != nil {
		t.Fatalf("pages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1: %+v", len(pages), pages)
	}
	if pages[0].URL != "/a%b/" {
		t.Fatalf("page = %+v", pages[0])
	}
}
