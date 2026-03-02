package store

import "testing"

func TestUpsertDoc(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	doc := &DocRow{
		URL:         "https://example.com/docs",
		SectionPath: "getting-started",
		Content:     "Hello world",
		SourceType:  "docs",
	}

	// First insert: should report changed.
	id, changed, err := st.UpsertDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("first UpsertDoc: changed = false, want true")
	}
	if id == 0 {
		t.Error("first UpsertDoc: id = 0")
	}

	// Same content: should report unchanged.
	id2, changed2, err := st.UpsertDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Error("duplicate UpsertDoc: changed = true, want false")
	}
	if id2 != id {
		t.Errorf("duplicate UpsertDoc: id = %d, want %d", id2, id)
	}

	// Updated content: should report changed.
	doc.Content = "Updated content"
	id3, changed3, err := st.UpsertDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !changed3 {
		t.Error("updated UpsertDoc: changed = false, want true")
	}

	// Verify content was updated.
	got, err := st.GetDoc(id3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "Updated content" {
		t.Errorf("GetDoc content = %q, want %q", got.Content, "Updated content")
	}
}

func TestSearchDocsFTS(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	docs := []DocRow{
		{URL: "https://example.com", SectionPath: "hooks", Content: "Claude Code hooks allow automation", SourceType: "docs"},
		{URL: "https://example.com", SectionPath: "skills", Content: "Skills provide reusable prompts", SourceType: "docs"},
		{URL: "https://example.com", SectionPath: "mcp", Content: "MCP servers extend Claude Code with tools", SourceType: "docs"},
	}
	for i := range docs {
		if _, _, err := st.UpsertDoc(&docs[i]); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("single word", func(t *testing.T) {
		t.Parallel()
		results, err := st.SearchDocsFTS("hooks", "", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Error("SearchDocsFTS(hooks): got 0 results, want >= 1")
		}
	})

	t.Run("multi word OR fallback", func(t *testing.T) {
		t.Parallel()
		results, err := st.SearchDocsFTS("hooks automation", "", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Error("SearchDocsFTS(hooks automation): got 0 results, want >= 1")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		t.Parallel()
		results, err := st.SearchDocsFTS("", "", 10)
		if err != nil {
			t.Fatal(err)
		}
		if results != nil {
			t.Errorf("SearchDocsFTS(empty): got %d results, want nil", len(results))
		}
	})

	t.Run("source type filter", func(t *testing.T) {
		t.Parallel()
		results, err := st.SearchDocsFTS("hooks", "docs", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Error("SearchDocsFTS with source_type: got 0 results, want >= 1")
		}
	})
}

func TestSanitizeFTS5Query(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "hello world", "hello world"},
		{"special chars", `hello "world" (test)`, "hello world test"},
		{"reserved word", "NOT hook", "hook*"},
		{"prefix expansion", "mcp", "mcp*"},
		{"long word no prefix", "automation", "automation"},
		{"empty after filter", "AND OR NOT", ""},
		{"leading dash", "-removed word", "removed word"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeFTS5Query(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFTS5Query(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDeleteExpiredDocs(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	// Insert a doc with TTL of 0 (already expired).
	_, err := st.db.Exec(`
		INSERT INTO docs (url, section_path, content, content_hash, source_type, crawled_at, ttl_days)
		VALUES ('https://old.com', 'expired', 'old content', 'hash1', 'docs', datetime('now', '-2 days'), 0)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a doc with long TTL (not expired).
	doc := &DocRow{
		URL:         "https://fresh.com",
		SectionPath: "fresh",
		Content:     "fresh content",
		SourceType:  "docs",
		TTLDays:     365,
	}
	if _, _, err := st.UpsertDoc(doc); err != nil {
		t.Fatal(err)
	}

	n, err := st.DeleteExpiredDocs()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("DeleteExpiredDocs: deleted %d, want 1", n)
	}

	// Fresh doc should remain.
	results, err := st.SearchDocsFTS("fresh", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("fresh doc should survive DeleteExpiredDocs")
	}
}
