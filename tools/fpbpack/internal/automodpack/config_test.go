package automodpack

import (
	"strings"
	"testing"
)

func TestParseAndRenderPreservesUnknownSettings(t *testing.T) {
	input := `
# comment is allowed
future-setting: "keep-me"
modpack-host: true
connection-mode: HOLEPUNCH
modpack {
  name: "FPBCraft"
  General {
    main {
      display-name: "Core"
      required: true
      default-selected: true
      from-server: ["mods/*.jar", "kubejs/**"]
      exclude: ["kubejs/server_scripts/**"]
      editable: ["config/**"]
      future-group-field: "also-keep"
    }
    shaders {
      description: "Shaders"
      requires: ["main"]
      compatible-platforms: []
    }
  }
}
`
	doc, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	cfg := doc.Config()
	if cfg.Name != "FPBCraft" || len(cfg.Categories) != 1 || len(cfg.Categories[0].Groups) != 2 {
		t.Fatalf("unexpected parsed config: %#v", cfg)
	}
	cfg.Settings.BandwidthLimit = 25
	cfg.Categories[0].Groups[1].DefaultSelected = true
	if err := doc.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	rendered := string(doc.Render())
	if !strings.Contains(rendered, "future-setting") || !strings.Contains(rendered, "future-group-field") {
		t.Fatalf("unknown settings were not preserved:\n%s", rendered)
	}
	again, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatal(err)
	}
	reparsed := again.Config()
	if reparsed.Settings.BandwidthLimit != 25 || !reparsed.Categories[0].Groups[1].DefaultSelected {
		t.Fatalf("round trip lost edits: %#v", reparsed)
	}
}

func TestValidateGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Categories[0].Groups = append(cfg.Categories[0].Groups,
		Group{ID: "visual", Requires: []string{"missing"}},
		Group{ID: "cycle-a", Requires: []string{"cycle-b"}},
		Group{ID: "cycle-b", Requires: []string{"cycle-a"}},
	)
	findings := Validate(cfg)
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.Code] = true
	}
	if !codes["requires_missing"] || !codes["requires_cycle"] {
		t.Fatalf("expected missing dependency and cycle findings, got %#v", findings)
	}
}

func TestApplyRejectsInvalidIdentity(t *testing.T) {
	doc, err := Parse([]byte("modpack { name: \"Pack\" General { main { required: true } } }"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := doc.Config()
	cfg.Categories[0].Groups = append(cfg.Categories[0].Groups, Group{ID: "../bad"})
	if err := doc.Apply(cfg); err == nil {
		t.Fatal("expected invalid group id to be rejected")
	}
}
