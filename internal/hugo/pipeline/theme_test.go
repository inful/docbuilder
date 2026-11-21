package pipeline

import (
	"strings"
	"testing"
)

func TestIndexContent(t *testing.T) {
	t.Run("relearn lists the children", func(t *testing.T) {
		body := indexContent("Title", "Description", false)
		if !strings.Contains(body, `{{% children description="true" %}}`) {
			t.Errorf("body does not list the children: %q", body)
		}
	})

	t.Run("docsy leaves the list to the theme", func(t *testing.T) {
		body := indexContent("Title", "Description", true)
		if strings.Contains(body, "children") {
			t.Errorf("body carries the Relearn children shortcode: %q", body)
		}
		if !strings.Contains(body, "# Title") || !strings.Contains(body, "Description") {
			t.Errorf("body lost its title or description: %q", body)
		}
	})
}

func TestIsDocsy(t *testing.T) {
	if IsDocsy(nil) {
		t.Error("a missing configuration is not the Docsy theme")
	}
}
