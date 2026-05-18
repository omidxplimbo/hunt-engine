package crawling

import "testing"

func TestCanonicalizeCrawledURLRemovesVolatileParams(t *testing.T) {
	first, firstHash := canonicalizeCrawledURL("https://developerportal.uscellular.com/g/login/auth?org.apache.catalina.filters.CSRF_NONCE=AAA&org=BBB&org.apache.catalina.filters.CSRF_NONCE=CCC")
	second, secondHash := canonicalizeCrawledURL("https://developerportal.uscellular.com/g/login/auth?org.apache.catalina.filters.CSRF_NONCE=DDD&org=EEE")

	if first != "https://developerportal.uscellular.com/g/login/auth?org" {
		t.Fatalf("unexpected canonical URL: %s", first)
	}

	if first != second {
		t.Fatalf("expected canonical URLs to match: %s != %s", first, second)
	}

	if firstHash == "" || firstHash != secondHash {
		t.Fatalf("expected matching stable hashes: %s != %s", firstHash, secondHash)
	}
}

func TestCanonicalizeCrawledURLKeepsParameterNamesButDropsValues(t *testing.T) {
	first, _ := canonicalizeCrawledURL("https://example.com/product?id=123&page=2")
	second, _ := canonicalizeCrawledURL("https://example.com/product?page=5&id=999")

	if first != "https://example.com/product?id&page" {
		t.Fatalf("unexpected canonical URL: %s", first)
	}

	if first != second {
		t.Fatalf("expected query order/value-insensitive canonical URLs: %s != %s", first, second)
	}
}

func TestCanonicalizeCrawledURLDropsFragmentsAndMarketingParams(t *testing.T) {
	canonical, _ := canonicalizeCrawledURL("https://Example.COM//admin///login/?utm_source=x&fbclid=y#section")

	if canonical != "https://example.com/admin/login" {
		t.Fatalf("unexpected canonical URL: %s", canonical)
	}
}

func TestIsVolatileURLParam(t *testing.T) {
	volatile := []string{
		"csrf_token",
		"org.apache.catalina.filters.csrf_nonce",
		"sessionid",
		"auth_token",
		"signature",
		"utm_campaign",
		"fbclid",
	}

	for _, key := range volatile {
		if !isVolatileURLParam(key) {
			t.Fatalf("expected %s to be volatile", key)
		}
	}

	stable := []string{"id", "redirect", "url", "file", "page", "search"}
	for _, key := range stable {
		if isVolatileURLParam(key) {
			t.Fatalf("expected %s to be stable", key)
		}
	}
}
