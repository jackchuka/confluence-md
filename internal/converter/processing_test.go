package converter

import "testing"

func TestExtractImageReferencesDeduplicates(t *testing.T) {
	conv := &Converter{}
	html := `<p><ac:image ri:filename="a.png"></ac:image></p>` +
		`<p><ac:image ri:filename="a.png"></ac:image></p>` +
		`<p><ac:image ri:filename="b.png"></ac:image></p>`

	refs := conv.extractImageReferences(html, "123", "https://example.atlassian.net")

	var names []string
	for _, ref := range refs {
		names = append(names, ref.FileName)
	}
	if len(refs) != 2 {
		t.Fatalf("expected one reference per distinct file, got %v", names)
	}
	if names[0] != "a.png" || names[1] != "b.png" {
		t.Fatalf("unexpected references: %v", names)
	}
}
