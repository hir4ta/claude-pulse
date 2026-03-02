package coach

import (
	"context"
	"fmt"

	"github.com/hir4ta/claude-pulse/internal/embedder"
	"github.com/hir4ta/claude-pulse/internal/store"
)

// SearchResult is a best-practice search result.
type SearchResult struct {
	SectionPath string `json:"section_path"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	Score       float64 `json:"score,omitempty"`
}

// SearchBestPractices searches the knowledge base for Claude Code best practices.
// Uses hybrid vector+FTS5 search when embedder is available, FTS5-only otherwise.
func SearchBestPractices(ctx context.Context, st *store.Store, emb *embedder.Embedder, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Try hybrid search if embedder is available.
	if emb != nil {
		queryVec, err := emb.EmbedForSearch(ctx, query)
		if err == nil {
			ftsQuery := store.SanitizeFTS5Query(query)
			matches, err := st.HybridSearch(queryVec, ftsQuery, "", limit, limit*4)
			if err == nil && len(matches) > 0 {
				return matchesToResults(st, matches)
			}
		}
	}

	// Fallback to FTS5-only.
	docs, err := st.SearchDocsFTS(query, "", limit)
	if err != nil {
		return nil, fmt.Errorf("coach: search docs: %w", err)
	}

	var results []SearchResult
	for _, d := range docs {
		results = append(results, SearchResult{
			SectionPath: d.SectionPath,
			Content:     d.Content,
			URL:         d.URL,
		})
	}
	return results, nil
}

func matchesToResults(st *store.Store, matches []store.HybridMatch) ([]SearchResult, error) {
	ids := make([]int64, len(matches))
	scores := make(map[int64]float64)
	for i, m := range matches {
		ids[i] = m.DocID
		scores[m.DocID] = m.RRFScore
	}

	docs, err := st.GetDocsByIDs(ids)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, d := range docs {
		results = append(results, SearchResult{
			SectionPath: d.SectionPath,
			Content:     d.Content,
			URL:         d.URL,
			Score:       scores[d.ID],
		})
	}
	return results, nil
}
