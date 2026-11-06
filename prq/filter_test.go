//go:build !mock

package prq

import (
	"testing"
	"time"
)

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		name         string
		node         ghPRNode
		cfg          Config
		userReviewed bool
		requestAt    time.Time
		reviewAt     time.Time
		want         bool
	}{
		{
			name: "approved hidden by default",
			node: ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "APPROVED"},
			cfg:  Config{SkipApproved: true},
			want: false,
		},
		{
			name: "approved included when include-approved",
			node: ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "APPROVED"},
			cfg:  Config{SkipApproved: false},
			want: true,
		},
		{
			name:         "already reviewed still filtered",
			node:         ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "REVIEW_REQUIRED"},
			cfg:          Config{SkipAlreadyReviewed: true, SkipApproved: false},
			userReviewed: true,
			want:         false,
		},
		{
			name:         "re-requested review included even if already reviewed",
			node:         ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "REVIEW_REQUIRED"},
			cfg:          Config{SkipAlreadyReviewed: true, SkipApproved: false},
			userReviewed: true,
			reviewAt:     time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			requestAt:    time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC),
			want:         true,
		},
		{
			name: "bot still filtered",
			node: ghPRNode{Author: ghAuthor{TypeName: "Bot"}, ReviewDecision: "REVIEW_REQUIRED"},
			cfg:  Config{SkipBots: true, SkipApproved: false},
			want: false,
		},
		{
			name: "regular pr included",
			node: ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "REVIEW_REQUIRED"},
			cfg:  Config{SkipApproved: true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldInclude(tt.node, tt.cfg, tt.userReviewed, tt.requestAt, tt.reviewAt)
			if got != tt.want {
				t.Fatalf("shouldInclude() = %v; want %v", got, tt.want)
			}
		})
	}
}
