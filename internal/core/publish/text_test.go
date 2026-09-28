package publish

import "testing"

func TestPlainText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<p>Hola <strong>mundo</strong> (#1)</p><ul><li><p>uno</p></li></ul>", "Hola 𝗺𝘂𝗻𝗱𝗼 (#1)\n- uno"},
		{"<p>uno</p><p>dos</p>", "uno\ndos"},
		{`<p>Mira <a href="https://example.com/a?b=1&amp;c=2">esto</a></p>`, "Mira https://example.com/a?b=1&c=2"},
		{"<p><u>ab1</u> &amp; &lt;x&gt;</p>", "a̲b̲" + "1̲ & <x>"},
		{"<p><strong>A&amp;B</strong></p>", "𝗔&𝗕"},
		{"sin etiquetas", "sin etiquetas"},
	}
	for _, c := range cases {
		if got := PlainText(c.in); got != c.want {
			t.Errorf("PlainText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLinkedInCommentaryEscapesTheReservedCharacters(t *testing.T) {
	got := LinkedInCommentary(`<p>a\b &lt;c&gt; #d ~e _f |g [h] *i (j) {k} @l</p>`)
	want := `a\\b \<c\> \#d \~e \_f \|g \[h\] \*i \(j\) \{k\} \@l`
	if got != want {
		t.Fatalf("LinkedInCommentary = %q, want %q", got, want)
	}
}
