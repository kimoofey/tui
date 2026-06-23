package prq

import "testing"

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		name         string
		node         ghPRNode
		cfg          Config
		userReviewed bool
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
			name: "already reviewed still filtered",
			node: ghPRNode{Author: ghAuthor{TypeName: "User"}, ReviewDecision: "REVIEW_REQUIRED"},
			cfg:  Config{SkipAlreadyReviewed: true, SkipApproved: false},
			userReviewed: true,
			want: false,
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
			got := shouldInclude(tt.node, tt.cfg, tt.userReviewed)
			if got != tt.want {
				t.Fatalf("shouldInclude() = %v; want %v", got, tt.want)
			}
		})
	}
}
