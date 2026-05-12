package slim

import "testing"

func TestFlattenIdentity(t *testing.T) {
	if got := FlattenIdentity(nil); got != "Unknown" {
		t.Fatalf("nil => %q", got)
	}
	if got := FlattenIdentity("plain"); got != "plain" {
		t.Fatalf("string passthrough => %q", got)
	}
	got := FlattenIdentity(map[string]any{"displayName": "Alice", "uniqueName": "alice@x.com"})
	if got != "Alice <alice@x.com>" {
		t.Fatalf("with email => %q", got)
	}
	got = FlattenIdentity(map[string]any{"displayName": "Alice"})
	if got != "Alice" {
		t.Fatalf("no email => %q", got)
	}
	got = FlattenIdentity(map[string]any{"displayName": "alice@x.com", "uniqueName": "alice@x.com"})
	if got != "alice@x.com" {
		t.Fatalf("equal name/email => %q", got)
	}
}

func TestStripHtml(t *testing.T) {
	in := "<p>Hello <b>world</b></p><ul><li>item1<li>item2</ul>"
	out := StripHtml(in)
	if out == "" {
		t.Fatalf("empty out")
	}
	if out != "Hello world\n- item1- item2" {
		t.Fatalf("got %q", out)
	}
	if StripHtml("&amp;&lt;&gt;&quot;&#39;&nbsp;X") != "&<>\"' X" {
		t.Fatalf("entities mismatch: %q", StripHtml("&amp;&lt;&gt;&quot;&#39;&nbsp;X"))
	}
}

func TestStripRefPrefix(t *testing.T) {
	if StripRefPrefix("refs/heads/main") != "main" {
		t.Fail()
	}
	if StripRefPrefix("main") != "main" {
		t.Fail()
	}
	if StripRefPrefix("") != "" {
		t.Fail()
	}
}

func TestMapVote(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want string
	}{
		{nil, "noVote"},
		{0, "noVote"},
		{10, "approved"},
		{-10, "rejected"},
		{99, "unknown(99)"},
	} {
		if got := MapVote(tc.in); got != tc.want {
			t.Errorf("MapVote(%v) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapChangeType(t *testing.T) {
	if MapChangeType(1) != "add" {
		t.Fail()
	}
	if MapChangeType(16) != "delete" {
		t.Fail()
	}
	if MapChangeType("Edit") != "edit" {
		t.Fail()
	}
	if MapChangeType(nil) != "unknown" {
		t.Fail()
	}
	if MapChangeType(9999) != "unknown(9999)" {
		t.Fail()
	}
}

func TestExtractWorkItemIDFromURL(t *testing.T) {
	if ExtractWorkItemIDFromURL("https://x/_apis/wit/workItems/123") != 123 {
		t.Fail()
	}
	if ExtractWorkItemIDFromURL("nope") != -1 {
		t.Fail()
	}
}

func TestFriendlyFieldName(t *testing.T) {
	if FriendlyFieldName("System.Title") != "Title" {
		t.Fail()
	}
	if FriendlyFieldName("Microsoft.VSTS.TCM.ReproSteps") != "ReproSteps" {
		t.Fail()
	}
	if FriendlyFieldName("Custom.Foo") != "Custom.Foo" {
		t.Fail()
	}
}

func TestFirstLineShortSha(t *testing.T) {
	if FirstLine("a\nb\nc") != "a" {
		t.Fail()
	}
	if FirstLine("only") != "only" {
		t.Fail()
	}
	if ShortSha("abcdef0123456789") != "abcdef01" {
		t.Fail()
	}
	if ShortSha("abc") != "abc" {
		t.Fail()
	}
}
