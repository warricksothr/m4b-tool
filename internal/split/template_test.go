package split

import "testing"

func TestRender_Default(t *testing.T) {
	tmpl, err := CompileFilenameTemplate("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(tmpl, TemplateData{Track: 7, Title: "The Dragon Reborn"})
	if err != nil {
		t.Fatal(err)
	}
	want := "007-The Dragon Reborn"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_CustomTemplateAndAllFields(t *testing.T) {
	tmpl, err := CompileFilenameTemplate(`{{.Artist}} - {{.Album}} - {{printf "%02d" .Track}} - {{.Title}}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(tmpl, TemplateData{
		Track:  3,
		Title:  "Words of Radiance",
		Album:  "Stormlight Archive",
		Artist: "Brandon Sanderson",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Brandon Sanderson - Stormlight Archive - 03 - Words of Radiance"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_SanitizesPathSeparators(t *testing.T) {
	tmpl, _ := CompileFilenameTemplate("{{.Title}}")
	got, _ := Render(tmpl, TemplateData{Track: 1, Title: "AC/DC: Live?"})
	// Path separator and Windows-illegal chars become _; trimmed at edges.
	if got != "AC_DC_ Live_" {
		t.Errorf("got %q", got)
	}
}

func TestRender_FallbackOnEmptyResult(t *testing.T) {
	tmpl, _ := CompileFilenameTemplate("{{.Title}}")
	got, _ := Render(tmpl, TemplateData{Track: 5, Title: ""})
	if got != "005" {
		t.Errorf("expected fallback to track number, got %q", got)
	}
}

func TestCompileFilenameTemplate_BadSyntax(t *testing.T) {
	if _, err := CompileFilenameTemplate(`{{.Title`); err == nil {
		t.Errorf("expected parse error")
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"normal":          "normal",
		"a/b\\c:d*e?f":    "a_b_c_d_e_f",
		"   spaced   ":    "spaced",
		"...dotted...":    "dotted",
		"":                "",
		"control\x07char": "control_char",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
