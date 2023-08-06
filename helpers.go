package c3po

import "strings"

var htmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	// "&#34;" is shorter than "&quot;".
	`"`, "&#34;",
	// "&#39;" is shorter than "&apos;" and apos was not in HTML until HTML5.
	"'", "&#39;",
)

func HtmlEscape(s string) string { return htmlReplacer.Replace(s) }

func try() bool {
	err := recover()
	return err != nil
}

func RetInvalidType(f *Fielder) map[string]any {
	d := map[string]any{
		"message":  "invalid type",
		"required": f.Required,
	}

	if f.Name != "" {
		d["field"] = f.Name
	}

	return d
}

func RetMissing(f *Fielder) map[string]any {
	d := map[string]any{
		"message":  "missing",
		"required": f.Required,
	}
	if f.Name != "" {
		d["field"] = f.Name
	}

	return d
}
