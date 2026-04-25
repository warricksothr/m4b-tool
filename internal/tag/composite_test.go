package tag

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/warricksothr/m4b-tool/internal/audio"
)

type fakeImporter struct {
	name    string
	mutate  func(audio.Tag) audio.Tag
	err     error
	invoked *bool
}

func (f *fakeImporter) Name() string { return f.name }
func (f *fakeImporter) Improve(_ context.Context, t audio.Tag, _ string) (audio.Tag, error) {
	if f.invoked != nil {
		*f.invoked = true
	}
	if f.err != nil {
		return t, f.err
	}
	if f.mutate != nil {
		t = f.mutate(t)
	}
	return t, nil
}

func TestComposite_RunsInOrder(t *testing.T) {
	c := &Composite{
		Importers: []Importer{
			&fakeImporter{name: "a", mutate: func(t audio.Tag) audio.Tag { t.Title = "from-a"; return t }},
			&fakeImporter{name: "b", mutate: func(t audio.Tag) audio.Tag { t.Title += "+b"; return t }},
		},
	}
	out, err := c.Improve(context.Background(), audio.Tag{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "from-a+b" {
		t.Errorf("got %q, want from-a+b", out.Title)
	}
}

func TestComposite_DisableSkips(t *testing.T) {
	aRan, bRan := false, false
	c := &Composite{
		Importers: []Importer{
			&fakeImporter{name: "a", invoked: &aRan},
			&fakeImporter{name: "b", invoked: &bRan},
		},
		Disable: NamesSet([]string{"A"}),
	}
	if _, err := c.Improve(context.Background(), audio.Tag{}, ""); err != nil {
		t.Fatal(err)
	}
	if aRan {
		t.Errorf("a ran despite being disabled")
	}
	if !bRan {
		t.Errorf("b did not run")
	}
}

func TestComposite_EnableWhitelist(t *testing.T) {
	aRan, bRan := false, false
	c := &Composite{
		Importers: []Importer{
			&fakeImporter{name: "a", invoked: &aRan},
			&fakeImporter{name: "b", invoked: &bRan},
		},
		Enable: NamesSet([]string{"b"}),
	}
	if _, err := c.Improve(context.Background(), audio.Tag{}, ""); err != nil {
		t.Fatal(err)
	}
	if aRan {
		t.Errorf("a ran outside the whitelist")
	}
	if !bRan {
		t.Errorf("b didn't run despite being whitelisted")
	}
}

func TestComposite_DisableWinsOverEnable(t *testing.T) {
	aRan := false
	c := &Composite{
		Importers: []Importer{&fakeImporter{name: "a", invoked: &aRan}},
		Enable:    NamesSet([]string{"a"}),
		Disable:   NamesSet([]string{"a"}),
	}
	if _, err := c.Improve(context.Background(), audio.Tag{}, ""); err != nil {
		t.Fatal(err)
	}
	if aRan {
		t.Errorf("disable should win over enable")
	}
}

func TestComposite_ErrorAborts(t *testing.T) {
	bRan := false
	errBoom := errors.New("boom")
	c := &Composite{
		Importers: []Importer{
			&fakeImporter{name: "a", err: errBoom},
			&fakeImporter{name: "b", invoked: &bRan},
		},
	}
	_, err := c.Improve(context.Background(), audio.Tag{}, "")
	if err == nil || !errors.Is(err, errBoom) {
		t.Errorf("err = %v, want wrapping boom", err)
	}
	if bRan {
		t.Errorf("b should not have run after a errored")
	}
}

func TestValidateNames(t *testing.T) {
	if err := ValidateNames([]string{"FFMetadata", "cover", "cuesheet"}); err != nil {
		t.Errorf("should accept known names: %v", err)
	}
	if err := ValidateNames([]string{"ffmetadata", "bogus"}); err == nil {
		t.Error("expected error for bogus name")
	} else if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("err missing bad name: %v", err)
	}
	if err := ValidateNames(nil); err != nil {
		t.Errorf("nil input should return nil: %v", err)
	}
}
