package ikman

import (
	"encoding/json"
	"testing"
)

func TestExtractInitialData(t *testing.T) {
	html := []byte(`<script>window.initialData = {"serp":{"ads":{"type":"Success","data":{"ads":[{"title":"A \"quoted\" ad"}]}}}};</script>`)

	payload, err := ExtractInitialData(html)
	if err != nil {
		t.Fatalf("ExtractInitialData() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
}

func TestPriceValue(t *testing.T) {
	if got := PriceValue("Rs 1,250,000 per perch"); got != 1250000 {
		t.Fatalf("PriceValue() = %d, want 1250000", got)
	}
}

func TestExtractPublicPhone(t *testing.T) {
	tests := map[string]string{
		"Call 0700000000":        "0700000000",
		"WhatsApp 77 000 0000":   "77 000 0000",
		"Price Rs 110,000 only":  "",
		"Contact +94 77 0000000": "+94 77 0000000",
	}
	for input, want := range tests {
		if got := extractPublicPhone(input); got != want {
			t.Fatalf("extractPublicPhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAdSummaryAcceptsObjectShapedIsJobAd(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "bool true", raw: `true`, want: true},
		{name: "bool false", raw: `false`, want: false},
		{name: "empty object", raw: `{}`, want: false},
		{name: "object value true", raw: `{"value":true}`, want: true},
		{name: "object without value", raw: `{"label":"Job"}`, want: true},
		{name: "string true", raw: `"true"`, want: true},
		{name: "null", raw: `null`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ad AdSummary
			if err := json.Unmarshal([]byte(`{"title":"Sample","isJobAd":`+tt.raw+`}`), &ad); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if ad.IsJobAd != tt.want {
				t.Fatalf("IsJobAd = %v, want %v", ad.IsJobAd, tt.want)
			}
		})
	}
}

func TestDetailGalleryDedupesImageVariants(t *testing.T) {
	firstID := "11111111-1111-1111-1111-111111111111"
	secondID := "22222222-2222-2222-2222-222222222222"
	thirdID := "33333333-3333-3333-3333-333333333333"
	ad := DetailAd{
		Slug: "sample-ad",
		Images: ImageSet{
			BaseURI: "https://i.ikman-st.com",
			Meta: []ImageMeta{
				{Src: "https://i.ikman-st.com/sample-ad/" + firstID},
			},
			IDs: []string{firstID, secondID},
		},
		ImgURL: "https://i.ikman-st.com/sample-ad/" + firstID + "/142/107/cropped.jpg",
	}

	ad.SetGalleryURLs([]string{
		"https://i.ikman-st.com/sample-ad/" + firstID + "/1200/900/fitted.jpg",
		"https://i.ikman-st.com/sample-ad/" + thirdID + "/1200/900/fitted.jpg",
	})

	if got := ad.PhotoCount(); got != 3 {
		t.Fatalf("PhotoCount() = %d, want 3", got)
	}
	seen := map[string]bool{}
	for _, value := range ad.GalleryURLs() {
		id := ikmanImageID(value)
		if id == "" {
			t.Fatalf("GalleryURLs() included URL without image id: %q", value)
		}
		if seen[id] {
			t.Fatalf("GalleryURLs() included duplicate image id %q in %v", id, ad.GalleryURLs())
		}
		seen[id] = true
	}
	for _, id := range []string{firstID, secondID, thirdID} {
		if !seen[id] {
			t.Fatalf("GalleryURLs() missing image id %q in %v", id, ad.GalleryURLs())
		}
	}
}

func TestFilterAdsAdditionalFilters(t *testing.T) {
	ads := []AdSummary{
		{
			Title:              "Dealer phone",
			ShopName:           "Alpha Mobile",
			MembershipLevel:    "premium",
			IsAuthDealer:       true,
			IsDoorstepDelivery: true,
			IsDeliveryFree:     true,
			IsTopAd:            true,
			IsUrgentAd:         true,
			IsExtraImages:      true,
			Images: ImageSet{
				IDs: []string{"1", "2", "3"},
			},
			Category: Category{Name: "Mobile phones"},
			Location: "Colombo",
		},
		{
			Title:    "Private phone",
			ShopName: "Beta Seller",
			Images: ImageSet{
				IDs: []string{"1"},
			},
			Category: Category{Name: "Electronics"},
			Location: "Kandy",
		},
	}

	filtered := FilterAds(ads, SearchParams{
		Seller:         "alpha",
		AuthDealerOnly: true,
		DoorstepOnly:   true,
		FreeDelivery:   true,
		TopOnly:        true,
		UrgentOnly:     true,
		ExtraImages:    true,
		MinImages:      2,
	})

	if len(filtered) != 1 || filtered[0].Title != "Dealer phone" {
		t.Fatalf("FilterAds() = %#v, want only dealer phone", filtered)
	}

	if got := FilterAds(ads, SearchParams{Seller: "missing"}); len(got) != 0 {
		t.Fatalf("FilterAds() seller mismatch returned %d ads, want 0", len(got))
	}
}

func TestFilterAdsAdditionalSorts(t *testing.T) {
	ads := []AdSummary{
		{Title: "One", Location: "Kandy", Category: Category{Name: "Vehicles"}, Images: ImageSet{IDs: []string{"1"}}},
		{Title: "Two", Location: "Colombo", Category: Category{Name: "Electronics"}, Images: ImageSet{IDs: []string{"1", "2"}}},
	}

	if got := FilterAds(ads, SearchParams{Sort: "images_desc"}); got[0].Title != "Two" {
		t.Fatalf("images_desc first = %q, want Two", got[0].Title)
	}
	if got := FilterAds(ads, SearchParams{Sort: "location_asc"}); got[0].Title != "Two" {
		t.Fatalf("location_asc first = %q, want Two", got[0].Title)
	}
	if got := FilterAds(ads, SearchParams{Sort: "category_asc"}); got[0].Title != "Two" {
		t.Fatalf("category_asc first = %q, want Two", got[0].Title)
	}
}

func TestFilterAdsCalledFilter(t *testing.T) {
	ads := []AdSummary{
		{Title: "Called", Called: true},
		{Title: "Not called"},
	}

	if got := FilterAds(ads, SearchParams{CalledFilter: "hide"}); len(got) != 1 || got[0].Title != "Not called" {
		t.Fatalf("hide called = %#v, want only not called", got)
	}
	if got := FilterAds(ads, SearchParams{CalledFilter: "only"}); len(got) != 1 || got[0].Title != "Called" {
		t.Fatalf("only called = %#v, want only called", got)
	}
}
