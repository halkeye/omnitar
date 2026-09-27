package models

import "testing"

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  Slack@GavinMogan.com ": "slack@gavinmogan.com",
		"foo@bar.com":             "foo@bar.com",
		"":                        "",
	}
	for in, want := range cases {
		if got := NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMD5Hash(t *testing.T) {
	// Known Gravatar test vector: md5("test@example.com")
	const want = "55502f40dc8b7c769880b10874abc9d0"
	if got := MD5Hash("Test@Example.com "); got != want {
		t.Errorf("MD5Hash = %q, want %q", got, want)
	}
}

func TestSHA256Hash(t *testing.T) {
	// Known Gravatar test vector: sha256("test@example.com")
	const want = "973dfe463ec85785f5f95af5ba3906eedb2d931c24e69824a89ea65dba4e813b"
	if got := SHA256Hash("Test@Example.com "); got != want {
		t.Errorf("SHA256Hash = %q, want %q", got, want)
	}
}
