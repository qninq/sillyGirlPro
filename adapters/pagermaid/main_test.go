package pagermaid

import (
	"testing"

	"github.com/qninq/sillyGirlPro/core"
)

func TestValidAuthorization(t *testing.T) {
	if !validAuthorization("", "", "", "127.0.0.1:12345") {
		t.Fatal("empty token should allow local development connections")
	}
	if validAuthorization("", "", "", "192.168.1.20:12345") {
		t.Fatal("empty token should reject non-local connections")
	}
	if !validAuthorization("Bearer secret", "", "secret", "192.168.1.20:12345") {
		t.Fatal("bearer token should be accepted")
	}
	if !validAuthorization("", "secret", "secret", "192.168.1.20:12345") {
		t.Fatal("query token should be accepted")
	}
	if validAuthorization("Bearer wrong", "", "secret", "127.0.0.1:12345") {
		t.Fatal("wrong token should be rejected")
	}
}

func TestIsPrivateChat(t *testing.T) {
	if !isPrivateChat("private") {
		t.Fatal("private chat should be private")
	}
	if isPrivateChat("") {
		t.Fatal("missing chat type should not force group messages into private")
	}
	if isPrivateChat("supergroup") {
		t.Fatal("supergroup should not be private")
	}
}

func TestBooleanParsers(t *testing.T) {
	if !core.AdapterEnabledValue("") {
		t.Fatal("enable empty value should default to true")
	}
	if !core.AdapterEnabledValue("b:true") {
		t.Fatal("encoded b:true should be parsed as true")
	}
	if core.AdapterEnabledValue("b:false") {
		t.Fatal("encoded b:false should be parsed as false")
	}
	if truthyValue("") {
		t.Fatal("debug empty value should default to false")
	}
	if !truthyValue("true") {
		t.Fatal("true should be parsed as true")
	}
}

func TestSplitReplySegments(t *testing.T) {
	segments := splitReplySegments("hello [CQ:image,file=https://example.com/a&#44;b.png] world")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if segments[0].kind != replySegmentText || segments[0].value != "hello" {
		t.Fatalf("unexpected first segment: %#v", segments[0])
	}
	if segments[1].kind != replySegmentImage || segments[1].value != "https://example.com/a,b.png" {
		t.Fatalf("unexpected image segment: %#v", segments[1])
	}
	if segments[2].kind != replySegmentText || segments[2].value != "world" {
		t.Fatalf("unexpected last segment: %#v", segments[2])
	}
}
