package invites

import "testing"

func TestParseLinkCommand(t *testing.T) {
	code, ok := ParseLinkCommand("/link MB-ABC123")
	if !ok {
		t.Fatal("expected valid link command")
	}
	if code != "MB-ABC123" {
		t.Fatalf("unexpected code: %s", code)
	}

	if _, ok := ParseLinkCommand("hello"); ok {
		t.Fatal("expected invalid command")
	}
	if _, ok := ParseLinkCommand("/link"); ok {
		t.Fatal("expected invalid command for missing code")
	}
}

func TestHashCodeDeterministic(t *testing.T) {
	svc := &Service{pepper: "pepper"}
	a := svc.HashCode("mb-abcd")
	b := svc.HashCode("MB-ABCD")
	if a != b {
		t.Fatalf("hash should be case-insensitive and stable: %s != %s", a, b)
	}
}
