package main

import (
	"bytes"
	"testing"
)

func TestTemplatesComment(t *testing.T) {
	r := newTemplateRenderer()
	b := bytes.NewBufferString("")
	var data = struct {
		Opener string
		Owner  string
		RunUrl string
	}{
		Opener: "opener",
		Owner:  "owner",
		RunUrl: "url",
	}
	err := r.Render(b, "comment.md", data, nil)
	if err != nil {
		t.Fatal(err)
	}

	expect := `@opener @owner
Hi, I'm a bot.

Sorry, [This Workflow Run](url) is cancelled.
Because currently could not accept added at pull request.

If needed, please re-run [This Workflow Run](url)
`
	if b.String() != expect {
		t.Fatal(b.String())
	}
}

func TestTemplatesPostManifest(t *testing.T) {
	r := newTemplateRenderer()
	b := bytes.NewBufferString("")
	var data = struct {
		Manifest string
		State    string
	}{
		Manifest: "{}",
		State:    "http://example.org/",
	}
	err := r.Render(b, "post_manifest.html", data, nil)
	if err != nil {
		t.Fatal(err)
	}

	expect := `<form action="https://github.com/settings/apps/new" method="POST">
	<textarea name="manifest" hidden>{}</textarea>
	<input type="hidden" name="state" value="http://example.org/" />
</form>
<script>document.querySelector("form").submit()</script>

`
	if b.String() != expect {
		t.Fatal(b.String())
	}
}
