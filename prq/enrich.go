package prq

type enrichResult struct {
	Updates map[string]PullRequest
	Err     error
}

func mergeEnrichment(base []PullRequest, updates map[string]PullRequest) ([]PullRequest, int) {
	if len(updates) == 0 {
		return base, 0
	}
	merged := make([]PullRequest, len(base))
	copy(merged, base)

	count := 0
	for i := range merged {
		update, ok := updates[merged[i].URL]
		if !ok {
			continue
		}
		merged[i].WaitSince = update.WaitSince
		merged[i].Files = update.Files
		merged[i].FilesTruncated = update.FilesTruncated
		merged[i].Enriched = true
		count++
	}
	return merged, count
}

func groupReviewPRs(prs []PullRequest) []PullRequest {
	if len(prs) == 0 {
		return prs
	}
	grouped := make([]PullRequest, 0, len(prs))
	for _, bucket := range []Bucket{BucketDirect, BucketTeam, BucketWatch} {
		for _, pr := range prs {
			if pr.Bucket == bucket {
				grouped = append(grouped, pr)
			}
		}
	}
	for _, pr := range prs {
		if pr.Bucket != BucketDirect && pr.Bucket != BucketTeam && pr.Bucket != BucketWatch {
			grouped = append(grouped, pr)
		}
	}
	return grouped
}
