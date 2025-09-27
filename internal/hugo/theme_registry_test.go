package hugo

import (
	"testing"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

func TestThemesRegistered(t *testing.T) {
    want := map[config.Theme]string{
        config.ThemeHextra: "github.com/imfing/hextra",
        config.ThemeDocsy:  "github.com/google/docsy",
    }
    for k, path := range want {
        tm := th.Get(k)
        if tm == nil { t.Fatalf("theme %s not registered", k) }
        if tm.Features().ModulePath != path { t.Fatalf("theme %s path=%q want %q", k, tm.Features().ModulePath, path) }
    }
}
