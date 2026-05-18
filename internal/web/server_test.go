package web

import (
	"net/http/httptest"
	"testing"
)

func TestDetailBackURLPreservesSafeListURL(t *testing.T) {
	r := httptest.NewRequest("GET", "/ads/example?back=%2Fads%3Fq%3Diphone%26location%3Dcolombo%26dealer%3D1", nil)

	if got, want := detailBackURL(r), "/ads?q=iphone&location=colombo&dealer=1"; got != want {
		t.Fatalf("detailBackURL() = %q, want %q", got, want)
	}
}

func TestDetailBackURLRejectsUnsafeURL(t *testing.T) {
	r := httptest.NewRequest("GET", "/ads/example?back=https%3A%2F%2Fevil.example%2Fads%3Fq%3Dx", nil)

	if got, want := detailBackURL(r), "/ads"; got != want {
		t.Fatalf("detailBackURL() = %q, want %q", got, want)
	}
}

func TestDetailBackURLFallsBackToDetailQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/ads/example?q=iphone&location=colombo", nil)

	if got, want := detailBackURL(r), "/ads?location=colombo&q=iphone"; got != want {
		t.Fatalf("detailBackURL() = %q, want %q", got, want)
	}
}

func TestDetailURLCarriesEncodedBackURL(t *testing.T) {
	got := detailURL("sample-ad", "/ads?q=iphone&location=colombo")
	want := "/ads/sample-ad?back=%2Fads%3Fq%3Diphone%26location%3Dcolombo"
	if got != want {
		t.Fatalf("detailURL() = %q, want %q", got, want)
	}
}

func TestParseSearchParamsHandlesCheckboxPairs(t *testing.T) {
	server := NewServer(nil, Config{LoadPhonesByDefault: true})
	r := httptest.NewRequest("GET", "/ads?phones=0&phones=1&dealer=1&doorstep=1&free_delivery=1&top=1&urgent=1&extra_images=1&seller=alpha&min_images=3&called=hide", nil)

	params := server.parseSearchParams(r)
	if !params.LoadPhones {
		t.Fatal("LoadPhones = false, want true")
	}
	if !params.AuthDealerOnly || !params.DoorstepOnly || !params.FreeDelivery || !params.TopOnly || !params.UrgentOnly || !params.ExtraImages {
		t.Fatalf("new checkbox filters not parsed: %#v", params)
	}
	if params.Seller != "alpha" || params.MinImages != 3 {
		t.Fatalf("seller/min images parsed as %q/%d, want alpha/3", params.Seller, params.MinImages)
	}
	if params.CalledFilter != "hide" {
		t.Fatalf("CalledFilter = %q, want hide", params.CalledFilter)
	}
}
