package plugin

import "testing"

func TestParseConfluenceImage(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "with filename",
			html: `<ac:image ri:filename="image.png"></ac:image>`,
			want: "image.png",
		},
		{
			name: "missing filename",
			html: `<ac:image></ac:image>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseConfluenceImage(tt.html); got != tt.want {
				t.Fatalf("parseConfluenceImage(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

func TestExtractLanguageParameter(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "found",
			html: `<ac:parameter ac:name="language">go</ac:parameter>`,
			want: "go",
		},
		{
			name: "not found",
			html: `<ac:parameter ac:name="theme">dark</ac:parameter>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLanguageParameter(tt.html); got != tt.want {
				t.Fatalf("extractLanguageParameter(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

func TestExtractCodeContent(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "plain text body",
			html: `<ac:plain-text-body>console.log(&quot;hi&quot;)</ac:plain-text-body>`,
			want: `console.log("hi")`,
		},
		{
			name: "converted cdata",
			html: `<ac:plain-text-body><!--[CDATA[fmt.Println(&quot;ok&quot;)]]></ac:plain-text-body>`,
			want: `fmt.Println("ok")`,
		},
		{
			name: "missing body",
			html: `<div>no body</div>`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCodeContent(tt.html); got != tt.want {
				t.Fatalf("extractCodeContent(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}
