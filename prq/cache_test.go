package prq

import (
	"fmt"
	"testing"
	"time"
)

func TestApplyCachedEnrichment_UsesHeadRefOID(t *testing.T) {
	now := time.Now()
	cache := newEnrichmentCache()
	cache.set(PullRequest{
		URL:            "u1",
		HeadRefOID:     "sha1",
		WaitSince:      now.Add(-2 * time.Hour),
		Files:          []PRFile{{Path: "a.go", Additions: 1, Deletions: 1}},
		FilesTruncated: false,
		Enriched:       true,
	})

	base := []PullRequest{
		{URL: "u1", HeadRefOID: "sha1"},
		{URL: "u2", HeadRefOID: "sha2"},
	}

	updated, hits, misses := applyCachedEnrichment(base, cache)
	if hits != 1 {
		t.Fatalf("hits=%d; want 1", hits)
	}
	if len(misses) != 1 || misses[0].URL != "u2" {
		t.Fatalf("misses=%v; want only u2", misses)
	}
	if !updated[0].Enriched {
		t.Fatalf("updated[0].Enriched=false; want true")
	}
	if updated[1].Enriched {
		t.Fatalf("updated[1].Enriched=true; want false")
	}
}

func TestApplyCachedEnrichment_InvalidatesWhenHeadChanges(t *testing.T) {
	cache := newEnrichmentCache()
	cache.set(PullRequest{URL: "u1", HeadRefOID: "sha-old", Enriched: true})

	base := []PullRequest{{URL: "u1", HeadRefOID: "sha-new"}}
	updated, hits, misses := applyCachedEnrichment(base, cache)
	if hits != 0 {
		t.Fatalf("hits=%d; want 0", hits)
	}
	if len(misses) != 1 {
		t.Fatalf("misses=%d; want 1", len(misses))
	}
	if updated[0].Enriched {
		t.Fatalf("updated[0].Enriched=true; want false")
	}
}

func TestEnrichmentCache_LRUEviction(t *testing.T) {
	cache := newEnrichmentCache()

	overflow := 10
	total := maxSessionEnrichmentEntries + overflow
	for i := range total {
		cache.set(PullRequest{
			URL:        fmt.Sprintf("https://github.com/org/repo/pull/%d", i),
			HeadRefOID: fmt.Sprintf("sha-%d", i),
			Enriched:   true,
			Files:      []PRFile{{Path: "a.go", Additions: 1}},
		})
	}

	if len(cache.entries) != maxSessionEnrichmentEntries {
		t.Fatalf("cache size = %d; want %d", len(cache.entries), maxSessionEnrichmentEntries)
	}
	if len(cache.order) != maxSessionEnrichmentEntries {
		t.Fatalf("order size = %d; want %d", len(cache.order), maxSessionEnrichmentEntries)
	}

	for i := range overflow {
		pr := PullRequest{
			URL:        fmt.Sprintf("https://github.com/org/repo/pull/%d", i),
			HeadRefOID: fmt.Sprintf("sha-%d", i),
		}
		if _, ok := cache.get(pr); ok {
			t.Errorf("entry %d should have been evicted but was found", i)
		}
	}

	for i := overflow; i < total; i++ {
		pr := PullRequest{
			URL:        fmt.Sprintf("https://github.com/org/repo/pull/%d", i),
			HeadRefOID: fmt.Sprintf("sha-%d", i),
		}
		if _, ok := cache.get(pr); !ok {
			t.Errorf("entry %d should be present but was not found", i)
		}
	}
}
