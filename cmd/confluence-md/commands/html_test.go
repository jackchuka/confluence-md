package commands

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHTMLConvertEndsWithNewline(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "page.html")
	if err := os.WriteFile(input, []byte("<p>Hello</p>"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("file", func(t *testing.T) {
		output := filepath.Join(dir, "page.md")
		htmlOptions.output = output
		t.Cleanup(func() { htmlOptions.output = "" })

		if err := runHTMLConvert(htmlCmd, []string{input}); err != nil {
			t.Fatalf("runHTMLConvert returned error: %v", err)
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "Hello\n" {
			t.Fatalf("file output = %q, want %q", got, "Hello\n")
		}
	})

	t.Run("stdout", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout := os.Stdout
		os.Stdout = w
		runErr := runHTMLConvert(htmlCmd, []string{input})
		os.Stdout = stdout
		_ = w.Close()
		got, _ := io.ReadAll(r)

		if runErr != nil {
			t.Fatalf("runHTMLConvert returned error: %v", runErr)
		}
		if string(got) != "Hello\n" {
			t.Fatalf("stdout = %q, want %q", got, "Hello\n")
		}
	})
}

func TestEmailFlagHelpMatchesBehavior(t *testing.T) {
	for _, cmd := range []*cobra.Command{pageCmd, treeCmd} {
		flag := cmd.Flags().Lookup("email")
		if flag == nil {
			t.Fatalf("%s: email flag missing", cmd.Name())
		}
		if strings.Contains(flag.Usage, "default") {
			t.Fatalf("%s: email usage %q claims a default, but the flag is required", cmd.Name(), flag.Usage)
		}
		if !strings.Contains(flag.Usage, "required") {
			t.Fatalf("%s: email usage %q does not say it is required", cmd.Name(), flag.Usage)
		}
	}
}

func TestOutputNameTemplateHelpListsLabelNames(t *testing.T) {
	flag := pageCmd.Flags().Lookup("output-name-template")
	if !strings.Contains(flag.Usage, ".LabelNames") {
		t.Fatalf("usage %q does not mention .LabelNames", flag.Usage)
	}
}
