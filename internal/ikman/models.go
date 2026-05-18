package ikman

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const publicHost = "https://ikman.lk"

type SearchParams struct {
	Query          string
	LocationSlug   string
	CategorySlug   string
	AdType         string
	Sort           string
	Page           int
	MinPrice       int64
	MaxPrice       int64
	MemberOnly     bool
	VerifiedOnly   bool
	FeaturedOnly   bool
	WithImagesOnly bool
	AuthDealerOnly bool
	DoorstepOnly   bool
	FreeDelivery   bool
	TopOnly        bool
	UrgentOnly     bool
	ExtraImages    bool
	MinImages      int
	Seller         string
	CalledFilter   string
	LoadPhones     bool
}

func (p SearchParams) NormalizedPage() int {
	if p.Page < 1 {
		return 1
	}
	return p.Page
}

func (p SearchParams) ListingPath() string {
	location := cleanSlug(p.LocationSlug)
	category := cleanSlug(p.CategorySlug)

	path := "/en/ads"
	if location != "" || category != "" {
		if location == "" {
			location = "sri-lanka"
		}
		path += "/" + location
	}
	if category != "" {
		path += "/" + category
	}

	values := url.Values{}
	if query := strings.TrimSpace(p.Query); query != "" {
		values.Set("query", query)
	}
	if page := p.NormalizedPage(); page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

type SearchResult struct {
	Ads       []AdSummary
	Total     int
	TotalText string
	SourceURL string
	Category  Category
}

type Category struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	ParentID int       `json:"parentId"`
	Parent   *Category `json:"parent"`
}

type ImageSet struct {
	IDs      []string    `json:"ids"`
	BaseURI  string      `json:"base_uri"`
	Meta     []ImageMeta `json:"meta"`
	Metadata []ImageMeta `json:"metadata"`
}

type ImageMeta struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
}

type AdSummary struct {
	ID                 string   `json:"id"`
	Slug               string   `json:"slug"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Details            string   `json:"details"`
	Subtitle           string   `json:"subtitle"`
	ImgURL             string   `json:"imgUrl"`
	Images             ImageSet `json:"images"`
	Price              string   `json:"price"`
	Discount           int64    `json:"discount"`
	MRP                int64    `json:"mrp"`
	IsMember           bool     `json:"isMember"`
	IsAuthDealer       bool     `json:"isAuthDealer"`
	IsFeaturedMember   bool     `json:"isFeaturedMember"`
	IsFeaturedAd       bool     `json:"isFeaturedAd"`
	ShowFeaturedAdUI   bool     `json:"showFeaturedAdUI"`
	MembershipLevel    string   `json:"membershipLevel"`
	ShopName           string   `json:"shopName"`
	ShopLogoURL        string   `json:"shopLogoUrl"`
	IsDoorstepDelivery bool     `json:"isDoorstepDelivery"`
	IsExtraImages      bool     `json:"isExtraImages"`
	IsDeliveryFree     bool     `json:"isDeliveryFree"`
	IsTopAd            bool     `json:"isTopAd"`
	IsUrgentAd         bool     `json:"isUrgentAd"`
	TimeStamp          string   `json:"timeStamp"`
	LastBumpUpDate     string   `json:"lastBumpUpDate"`
	Category           Category `json:"category"`
	IsVerified         bool     `json:"isVerified"`
	IsJobAd            bool     `json:"isJobAd"`
	Location           string   `json:"location"`
	AdType             string   `json:"adType"`
	IsLocalJob         bool     `json:"isLocalJob"`
	Phone              string   `json:"-"`
	PhoneSource        string   `json:"-"`
	Called             bool     `json:"-"`
	CalledBefore       bool     `json:"-"`
	CalledBeforePhones []string `json:"-"`
}

func (a *AdSummary) UnmarshalJSON(data []byte) error {
	type adSummaryAlias AdSummary
	payload := struct {
		*adSummaryAlias
		IsJobAd json.RawMessage `json:"isJobAd"`
	}{
		adSummaryAlias: (*adSummaryAlias)(a),
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if len(payload.IsJobAd) > 0 {
		a.IsJobAd = flexibleBool(payload.IsJobAd)
	}
	return nil
}

func (a AdSummary) PublicURL() string {
	if a.Slug == "" {
		return publicHost + "/en/ads"
	}
	return publicHost + "/en/ad/" + a.Slug
}

func (a AdSummary) LocalDetailPath() string {
	return "/ads/" + url.PathEscape(a.Slug)
}

func (a AdSummary) PriceValue() int64 {
	return PriceValue(a.Price)
}

func (a AdSummary) DisplayPhone() string {
	if strings.TrimSpace(a.Phone) != "" {
		return a.Phone
	}
	return "Unavailable"
}

func (a AdSummary) ImageCount() int {
	if len(a.Images.IDs) > 0 {
		return len(a.Images.IDs)
	}
	if len(a.Images.Meta) > 0 {
		return len(a.Images.Meta)
	}
	if a.ImgURL != "" {
		return 1
	}
	return 0
}

func (a AdSummary) ThumbURL() string {
	if a.ImgURL != "" {
		return a.ImgURL
	}
	if len(a.Images.IDs) == 0 || a.Images.BaseURI == "" || a.Slug == "" {
		return ""
	}
	return imageURL(a.Images.BaseURI, a.Slug, a.Images.IDs[0], 142, 107, "cropped")
}

type Detail struct {
	Ad         DetailAd    `json:"ad"`
	SimilarAds []AdSummary `json:"similarAds"`
	SourceURL  string      `json:"-"`
}

type DetailAd struct {
	ID                 string         `json:"id"`
	Slug               string         `json:"slug"`
	URL                string         `json:"url"`
	Title              string         `json:"title"`
	Description        string         `json:"description"`
	Details            string         `json:"details"`
	Properties         []Property     `json:"properties"`
	IsMember           bool           `json:"isMember"`
	IsAuthDealer       bool           `json:"isAuthDealer"`
	IsFeaturedMember   bool           `json:"isFeaturedMember"`
	Type               string         `json:"type"`
	Status             string         `json:"status"`
	MembershipLevel    string         `json:"membershipLevel"`
	MemberSince        string         `json:"memberSince"`
	IsVerified         bool           `json:"isVerified"`
	Timestamp          string         `json:"timestamp"`
	AdDate             string         `json:"adDate"`
	Location           DetailNode     `json:"location"`
	Area               DetailNode     `json:"area"`
	Category           DetailCategory `json:"category"`
	ContactCard        ContactCard    `json:"contactCard"`
	Images             ImageSet       `json:"images"`
	Gallery            []string       `json:"-"`
	Shop               Shop           `json:"shop"`
	Money              Money          `json:"money"`
	ImgURL             string         `json:"imgUrl"`
	IsDoorstepDelivery bool           `json:"isDoorstepDelivery"`
	IsDeliveryFree     bool           `json:"isDeliveryFree"`
}

type Property struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Key      string `json:"key"`
	ValueKey string `json:"value_key"`
}

type DetailNode struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Slug     string      `json:"slug"`
	Link     string      `json:"link"`
	ParentID int         `json:"parentId"`
	Parent   *DetailNode `json:"parent"`
}

type DetailCategory struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	Slug     string          `json:"slug"`
	ParentID int             `json:"parentId"`
	Parent   *DetailCategory `json:"parent"`
}

type ContactCard struct {
	Name         string        `json:"name"`
	PhoneNumbers []PhoneNumber `json:"phoneNumbers"`
	ChatEnabled  bool          `json:"chatEnabled"`
}

type PhoneNumber struct {
	Number   string `json:"number"`
	Verified bool   `json:"verified"`
}

type Shop struct {
	ID                 string `json:"id"`
	Slug               string `json:"slug"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Logo               string `json:"logo"`
	ContactSectionLogo string `json:"contactSectionLogo"`
}

type Money struct {
	Label  string `json:"label"`
	Amount string `json:"amount"`
}

func (a DetailAd) PriceText() string {
	if a.Money.Amount != "" {
		return a.Money.Amount
	}
	return "Not specified"
}

func (a DetailAd) PhoneText() string {
	if len(a.ContactCard.PhoneNumbers) == 0 {
		return extractPublicPhone(a.Title + " " + a.Description + " " + a.Details)
	}
	values := make([]string, 0, len(a.ContactCard.PhoneNumbers))
	for _, phone := range a.ContactCard.PhoneNumbers {
		if phone.Number != "" {
			values = append(values, phone.Number)
		}
	}
	if len(values) == 0 {
		return "Unavailable"
	}
	return strings.Join(values, ", ")
}

func (a DetailAd) GalleryURLs() []string {
	if len(a.Gallery) > 0 {
		return a.Gallery
	}
	return a.buildGalleryURLs()
}

func (a DetailAd) PhotoCount() int {
	return len(a.GalleryURLs())
}

func (a DetailAd) buildGalleryURLs() []string {
	seenURLs := map[string]bool{}
	seenImageIDs := map[string]bool{}
	var urls []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if id := ikmanImageID(value); id != "" {
			if seenImageIDs[id] {
				return
			}
			seenImageIDs[id] = true
		}
		if seenURLs[value] {
			return
		}
		seenURLs[value] = true
		urls = append(urls, value)
	}
	for _, meta := range a.Images.Meta {
		if meta.Src != "" {
			add(strings.TrimRight(meta.Src, "/") + "/800/600/fitted.jpg")
		}
	}
	for _, id := range a.Images.IDs {
		add(imageURL(a.Images.BaseURI, a.Slug, id, 800, 600, "fitted"))
	}
	if a.ImgURL != "" {
		add(a.ImgURL)
	}
	return urls
}

func (a *DetailAd) SetGalleryURLs(extra []string) {
	seenURLs := map[string]bool{}
	seenImageIDs := map[string]bool{}
	var urls []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if id := ikmanImageID(value); id != "" {
			if seenImageIDs[id] {
				return
			}
			seenImageIDs[id] = true
		}
		if seenURLs[value] {
			return
		}
		seenURLs[value] = true
		urls = append(urls, value)
	}
	for _, value := range a.buildGalleryURLs() {
		add(value)
	}
	for _, value := range extra {
		add(value)
	}
	a.Gallery = urls
}

func (a DetailAd) PublicURL() string {
	if a.URL != "" {
		return strings.Replace(a.URL, "http://", "https://", 1)
	}
	if a.Slug == "" {
		return publicHost
	}
	return publicHost + "/en/ad/" + a.Slug
}

func FilterAds(ads []AdSummary, p SearchParams) []AdSummary {
	filtered := make([]AdSummary, 0, len(ads))
	for _, ad := range ads {
		if p.AdType != "" && ad.AdType != p.AdType {
			continue
		}
		if p.MemberOnly && !ad.IsMember {
			continue
		}
		if p.VerifiedOnly && !ad.IsVerified {
			continue
		}
		if p.FeaturedOnly && !ad.IsFeaturedAd && !ad.IsFeaturedMember {
			continue
		}
		if p.WithImagesOnly && ad.ImageCount() == 0 {
			continue
		}
		if p.AuthDealerOnly && !ad.IsAuthDealer {
			continue
		}
		if p.DoorstepOnly && !ad.IsDoorstepDelivery {
			continue
		}
		if p.FreeDelivery && !ad.IsDeliveryFree {
			continue
		}
		if p.TopOnly && !ad.IsTopAd {
			continue
		}
		if p.UrgentOnly && !ad.IsUrgentAd {
			continue
		}
		if p.ExtraImages && !ad.IsExtraImages {
			continue
		}
		switch p.CalledFilter {
		case "hide":
			if ad.Called {
				continue
			}
		case "only":
			if !ad.Called {
				continue
			}
		}
		if p.MinImages > 0 && ad.ImageCount() < p.MinImages {
			continue
		}
		if seller := strings.ToLower(strings.TrimSpace(p.Seller)); seller != "" {
			haystack := strings.ToLower(strings.Join([]string{ad.ShopName, ad.MembershipLevel}, " "))
			if !strings.Contains(haystack, seller) {
				continue
			}
		}
		price := ad.PriceValue()
		if p.MinPrice > 0 && price > 0 && price < p.MinPrice {
			continue
		}
		if p.MaxPrice > 0 && price > 0 && price > p.MaxPrice {
			continue
		}
		filtered = append(filtered, ad)
	}

	switch p.Sort {
	case "price_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].PriceValue() < filtered[j].PriceValue()
		})
	case "price_desc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].PriceValue() > filtered[j].PriceValue()
		})
	case "title_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Title) < strings.ToLower(filtered[j].Title)
		})
	case "images_desc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].ImageCount() > filtered[j].ImageCount()
		})
	case "location_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Location) < strings.ToLower(filtered[j].Location)
		})
	case "category_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Category.Name) < strings.ToLower(filtered[j].Category.Name)
		})
	}
	return filtered
}

func PriceValue(text string) int64 {
	re := regexp.MustCompile(`\d[\d,]*`)
	match := re.FindString(text)
	if match == "" {
		return 0
	}
	value, _ := strconv.ParseInt(strings.ReplaceAll(match, ",", ""), 10, 64)
	return value
}

func flexibleBool(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	switch value {
	case "", "null", "false", "0", `""`:
		return false
	case "true", "1":
		return true
	}

	var asBool bool
	if err := json.Unmarshal(raw, &asBool); err == nil {
		return asBool
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		switch strings.ToLower(strings.TrimSpace(asString)) {
		case "true", "1", "yes", "y", "on":
			return true
		default:
			return false
		}
	}
	var asNumber float64
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return asNumber != 0
	}
	var asObject map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObject); err == nil {
		if len(asObject) == 0 {
			return false
		}
		for _, key := range []string{"value", "enabled", "active", "isJobAd"} {
			if nested, ok := asObject[key]; ok {
				return flexibleBool(nested)
			}
		}
		return true
	}
	var asArray []json.RawMessage
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return len(asArray) > 0
	}
	return false
}

func EnrichSummaryFromDetail(ad *AdSummary, detail Detail) {
	phone := detail.Ad.PhoneText()
	if phone != "" && phone != "Unavailable" {
		ad.Phone = phone
		ad.PhoneSource = "detail"
		return
	}
	if phone := extractPublicPhone(ad.Title + " " + ad.Description + " " + ad.Details); phone != "" {
		ad.Phone = phone
		ad.PhoneSource = "listing"
	}
}

func imageURL(baseURI, slug, id string, width, height int, mode string) string {
	if baseURI == "" || slug == "" || id == "" {
		return ""
	}
	return strings.TrimRight(baseURI, "/") + "/" + slug + "/" + id + "/" + strconv.Itoa(width) + "/" + strconv.Itoa(height) + "/" + mode + ".jpg"
}

func ikmanImageID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if regexp.MustCompile(`^[a-f0-9-]{36}$`).MatchString(value) {
		return value
	}
	matches := regexp.MustCompile(`/([a-f0-9-]{36})(?:/|$)`).FindStringSubmatch(value)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func extractPublicPhone(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:\+94|0)\s*7\d[\s.-]?\d{3}[\s.-]?\d{4}`),
		regexp.MustCompile(`(?i)\b7\d[\s.-]?\d{3}[\s.-]?\d{4}\b`),
	}
	for _, pattern := range patterns {
		match := pattern.FindString(text)
		if match != "" {
			return strings.Join(strings.Fields(match), " ")
		}
	}
	return ""
}

func cleanSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, "/")
	value = strings.ReplaceAll(value, " ", "-")
	allowed := regexp.MustCompile(`[^a-z0-9-]`)
	value = allowed.ReplaceAllString(value, "")
	value = regexp.MustCompile(`-+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
