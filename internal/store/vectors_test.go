package store

import (
	"math"
	"testing"
)

func TestSerializeDeserializeRoundtrip(t *testing.T) {
	t.Parallel()

	original := []float32{1.0, -2.5, 3.14, 0, math.SmallestNonzeroFloat32}
	blob := serializeFloat32(original)
	restored := deserializeFloat32(blob)

	if len(restored) != len(original) {
		t.Fatalf("length = %d, want %d", len(restored), len(original))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Errorf("[%d] = %g, want %g", i, restored[i], original[i])
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 1.0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0.0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1.0},
		{"empty", nil, nil, 0.0},
		{"length mismatch", []float32{1}, []float32{1, 2}, 0.0},
		{"zero vector", []float32{0, 0}, []float32{1, 1}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("cosineSimilarity(%v, %v) = %g, want %g", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVectorSearch(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	// Insert a doc to have a valid source_id.
	doc := &DocRow{URL: "https://example.com", SectionPath: "test", Content: "test content", SourceType: "docs"}
	id, _, err := st.UpsertDoc(doc)
	if err != nil {
		t.Fatal(err)
	}

	vec := []float32{1.0, 0.0, 0.0}
	if err := st.InsertEmbedding("docs", id, "test-model", vec); err != nil {
		t.Fatal(err)
	}

	// Query with similar vector (should match).
	query := []float32{0.9, 0.1, 0.0}
	matches, err := st.VectorSearch(query, "docs", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("VectorSearch: got 0 matches, want >= 1")
	}
	if matches[0].SourceID != id {
		t.Errorf("match source_id = %d, want %d", matches[0].SourceID, id)
	}

	// Query with orthogonal vector (below threshold).
	orthogonal := []float32{0.0, 0.0, 1.0}
	matches2, err := st.VectorSearch(orthogonal, "docs", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches2) != 0 {
		t.Errorf("orthogonal VectorSearch: got %d matches, want 0", len(matches2))
	}

	// Nil query.
	matches3, err := st.VectorSearch(nil, "docs", 10)
	if err != nil {
		t.Fatal(err)
	}
	if matches3 != nil {
		t.Errorf("nil VectorSearch: got %d matches, want nil", len(matches3))
	}
}

func TestHybridSearch(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	// Insert docs.
	docs := []DocRow{
		{URL: "https://example.com", SectionPath: "hooks", Content: "Claude Code hooks enable automation workflows", SourceType: "docs"},
		{URL: "https://example.com", SectionPath: "skills", Content: "Skills provide reusable prompt templates", SourceType: "docs"},
	}
	ids := make([]int64, len(docs))
	for i := range docs {
		id, _, err := st.UpsertDoc(&docs[i])
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}

	// Insert embeddings (hooks doc gets high similarity, skills doc gets low).
	if err := st.InsertEmbedding("docs", ids[0], "test", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertEmbedding("docs", ids[1], "test", []float32{0.0, 1.0, 0.0}); err != nil {
		t.Fatal(err)
	}

	// Hybrid search: vector favors hooks doc, FTS query also matches hooks.
	queryVec := []float32{0.9, 0.1, 0.0}
	results, err := st.HybridSearch(queryVec, "hooks", "", 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("HybridSearch: got 0 results, want >= 1")
	}
	// First result should be the hooks doc (highest RRF score from both sources).
	if results[0].DocID != ids[0] {
		t.Errorf("top result DocID = %d, want %d (hooks doc)", results[0].DocID, ids[0])
	}
}
