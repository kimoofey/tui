package prq

import "fmt"

const maxSessionEnrichmentEntries = 500

type enrichmentCache struct {
	entries map[string]PullRequest
	order   []string
}

func newEnrichmentCache() enrichmentCache {
	return enrichmentCache{entries: make(map[string]PullRequest)}
}

func cacheKey(pr PullRequest) string {
	return fmt.Sprintf("%s|%s", pr.URL, pr.HeadRefOID)
}

func (c *enrichmentCache) get(pr PullRequest) (PullRequest, bool) {
	key := cacheKey(pr)
	if key == "|" {
		return PullRequest{}, false
	}
	v, ok := c.entries[key]
	return v, ok
}

func (c *enrichmentCache) set(pr PullRequest) {
	key := cacheKey(pr)
	if key == "|" {
		return
	}
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = pr
	for len(c.order) > maxSessionEnrichmentEntries {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evict)
	}
}

func applyCachedEnrichment(base []PullRequest, cache enrichmentCache) (updated []PullRequest, hits int, misses []PullRequest) {
	if len(base) == 0 {
		return base, 0, nil
	}
	updated = make([]PullRequest, len(base))
	copy(updated, base)

	for i := range updated {
		cached, ok := cache.get(updated[i])
		if !ok || !cached.Enriched {
			misses = append(misses, updated[i])
			continue
		}
		updated[i].WaitSince = cached.WaitSince
		updated[i].Files = cached.Files
		updated[i].FilesTruncated = cached.FilesTruncated
		updated[i].Enriched = true
		hits++
	}
	return updated, hits, misses
}

func updateCacheFromPRs(cache *enrichmentCache, prs []PullRequest) {
	for _, pr := range prs {
		if pr.Enriched {
			cache.set(pr)
		}
	}
}
