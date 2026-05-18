package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ikmanbrowser/internal/calls"
	"ikmanbrowser/internal/ikman"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Config struct {
	LoadPhonesByDefault bool
	CallStore           *calls.Store
}

type Server struct {
	client              *ikman.Client
	templates           *template.Template
	loadPhonesByDefault bool
	callStore           *calls.Store
}

type Option struct {
	Label string
	Value string
}

type ListView struct {
	Params          ikman.SearchParams
	Ads             []ikman.AdSummary
	Result          ikman.SearchResult
	Error           string
	CategoryOptions []Option
	LocationOptions []Option
	AdTypeOptions   []Option
	SortOptions     []Option
	CalledOptions   []Option
	PrevURL         string
	NextURL         string
	ReturnURL       string
	PhoneNotice     string
	Duration        string
	LoadMoreURL     string
	SkippedPages    []int
}

type DetailView struct {
	Detail  ikman.Detail
	Error   string
	BackURL string
}

type EndpointsView struct {
	BaseURL string
}

func NewServer(client *ikman.Client, cfg Config) *Server {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"selected":  selected,
		"checked":   checked,
		"detailURL": detailURL,
		"phoneCSS":  phoneCSS,
		"nl2br":     nl2br,
	}).ParseFS(assets, "templates/*.html"))

	return &Server{
		client:              client,
		templates:           tmpl,
		loadPhonesByDefault: cfg.LoadPhonesByDefault,
		callStore:           cfg.CallStore,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	staticFS, _ := fs.Sub(assets, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/api/ads", s.ads)
	mux.HandleFunc("/api/called-states", s.calledStates)
	mux.HandleFunc("/api/called/", s.called)
	mux.HandleFunc("/api/phone/", s.phone)
	mux.HandleFunc("/api/preview/", s.preview)
	mux.HandleFunc("/ads", s.list)
	mux.HandleFunc("/ads/", s.detail)
	mux.HandleFunc("/endpoints", s.endpoints)
	return mux
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ads", http.StatusFound)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	params := s.parseSearchParams(r)
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	result, err := s.client.Search(ctx, params)
	s.annotateCallState(result.Ads)
	ads := ikman.FilterAds(result.Ads, params)
	phoneNotice := ""
	if err == nil && params.LoadPhones && len(ads) > 0 {
		phoneNotice = "Phones load progressively from public detail pages."
	} else if !params.LoadPhones {
		phoneNotice = "Phone loading is off; open details or enable phones."
	}

	view := ListView{
		Params:          params,
		Ads:             ads,
		Result:          result,
		CategoryOptions: categoryOptions(),
		LocationOptions: locationOptions(),
		AdTypeOptions:   adTypeOptions(),
		SortOptions:     sortOptions(),
		CalledOptions:   calledOptions(),
		PrevURL:         pageURL(r, params.NormalizedPage()-1),
		NextURL:         pageURL(r, params.NormalizedPage()+1),
		ReturnURL:       listReturnURL(r),
		PhoneNotice:     phoneNotice,
		Duration:        time.Since(started).Round(time.Millisecond).String(),
		LoadMoreURL:     nextAdsURL(r, params.NormalizedPage()+1),
	}
	if err != nil {
		view.Error = err.Error()
	}
	s.render(w, "list.html", view)
}

func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/ads/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	detail, err := s.client.Detail(ctx, slug)
	view := DetailView{Detail: detail, BackURL: detailBackURL(r)}
	if err != nil {
		view.Error = err.Error()
	}
	s.render(w, "detail.html", view)
}

func (s *Server) ads(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	params := s.parseSearchParams(r)
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	result, ads, servedPage, skipped, err := s.searchScrollablePage(ctx, params)
	view := ListView{
		Params:       params,
		Ads:          ads,
		Result:       result,
		ReturnURL:    listReturnURL(r),
		Duration:     time.Since(started).Round(time.Millisecond).String(),
		LoadMoreURL:  nextAdsURL(r, servedPage+1),
		SkippedPages: skipped,
	}

	var rows bytes.Buffer
	if err == nil {
		err = s.templates.ExecuteTemplate(&rows, "ad_rows", view)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rows":     rows.String(),
		"count":    len(ads),
		"page":     servedPage,
		"next":     view.LoadMoreURL,
		"skipped":  skipped,
		"duration": view.Duration,
	})
}

func (s *Server) searchScrollablePage(ctx context.Context, params ikman.SearchParams) (ikman.SearchResult, []ikman.AdSummary, int, []int, error) {
	var lastResult ikman.SearchResult
	var lastErr error
	var skipped []int
	for attempt := 0; attempt < 5; attempt++ {
		result, err := s.client.Search(ctx, params)
		lastResult = result
		if err == nil {
			s.annotateCallState(result.Ads)
			return result, ikman.FilterAds(result.Ads, params), params.NormalizedPage(), skipped, nil
		}
		lastErr = err
		skipped = append(skipped, params.NormalizedPage())
		params.Page = params.NormalizedPage() + 1
	}
	return lastResult, nil, params.NormalizedPage(), skipped, lastErr
}

func (s *Server) phone(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/phone/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	detail, err := s.client.Detail(ctx, slug)
	state := calls.State{}
	if err == nil {
		state = s.callState(slug, detailPhones(detail))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"phone":               detail.Ad.PhoneText(),
		"source":              "detail",
		"called":              state.Called,
		"calledBefore":        state.CalledBefore,
		"calledBeforeNumbers": state.CalledBeforeNumbers,
	})
}

func (s *Server) called(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/api/called/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	var request struct {
		Called bool   `json:"called"`
		Phone  string `json:"phone"`
		Title  string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)

	title := strings.TrimSpace(request.Title)
	phones := calls.SplitPhones(request.Phone)
	if request.Called {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if detail, err := s.client.Detail(ctx, slug); err == nil {
			title = firstNonEmpty(detail.Ad.Title, title)
			phones = detailPhones(detail)
		}
	}
	var state calls.State
	var err error
	if s.callStore != nil {
		state, err = s.callStore.Mark(slug, title, phones, request.Called)
	} else {
		state = calls.State{Called: request.Called}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"called":              state.Called,
		"calledBefore":        state.CalledBefore,
		"calledBeforeNumbers": state.CalledBeforeNumbers,
		"phone":               strings.Join(phones, ", "),
	})
}

func (s *Server) calledStates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Items []struct {
			Slug  string `json:"slug"`
			Phone string `json:"phone"`
		} `json:"items"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)

	states := map[string]calls.State{}
	for _, item := range request.Items {
		slug := strings.TrimSpace(item.Slug)
		if slug == "" {
			continue
		}
		states[slug] = s.callState(slug, calls.SplitPhones(item.Phone))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"states": states})
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/preview/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	detail, err := s.client.Detail(ctx, slug)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<div class="preview-error">Could not load preview.</div>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "ad_preview", DetailView{Detail: detail}); err != nil {
		log.Printf("render ad_preview: %v", err)
	}
}

func (s *Server) endpoints(w http.ResponseWriter, r *http.Request) {
	s.render(w, "endpoints.html", EndpointsView{BaseURL: "https://ikman.lk"})
}

func (s *Server) parseSearchParams(r *http.Request) ikman.SearchParams {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	minPrice, _ := strconv.ParseInt(strings.ReplaceAll(q.Get("min_price"), ",", ""), 10, 64)
	maxPrice, _ := strconv.ParseInt(strings.ReplaceAll(q.Get("max_price"), ",", ""), 10, 64)
	minImages, _ := strconv.Atoi(q.Get("min_images"))

	loadPhones := s.loadPhonesByDefault
	if q.Has("phones") {
		loadPhones = queryBool(q, "phones", s.loadPhonesByDefault)
	}

	return ikman.SearchParams{
		Query:          q.Get("q"),
		Seller:         q.Get("seller"),
		LocationSlug:   q.Get("location"),
		CategorySlug:   q.Get("category"),
		AdType:         q.Get("ad_type"),
		Sort:           q.Get("sort"),
		CalledFilter:   q.Get("called"),
		Page:           page,
		MinPrice:       minPrice,
		MaxPrice:       maxPrice,
		MemberOnly:     queryBool(q, "member", false),
		VerifiedOnly:   queryBool(q, "verified", false),
		FeaturedOnly:   queryBool(q, "featured", false),
		WithImagesOnly: queryBool(q, "images", false),
		AuthDealerOnly: queryBool(q, "dealer", false),
		DoorstepOnly:   queryBool(q, "doorstep", false),
		FreeDelivery:   queryBool(q, "free_delivery", false),
		TopOnly:        queryBool(q, "top", false),
		UrgentOnly:     queryBool(q, "urgent", false),
		ExtraImages:    queryBool(q, "extra_images", false),
		MinImages:      minImages,
		LoadPhones:     loadPhones,
	}
}

func (s *Server) annotateCallState(ads []ikman.AdSummary) {
	if s.callStore == nil {
		return
	}
	for i := range ads {
		state := s.callState(ads[i].Slug, calls.SplitPhones(ads[i].Phone))
		ads[i].Called = state.Called
		ads[i].CalledBefore = state.CalledBefore
		ads[i].CalledBeforePhones = state.CalledBeforeNumbers
	}
}

func (s *Server) callState(slug string, phones []string) calls.State {
	if s.callStore == nil {
		return calls.State{}
	}
	return s.callStore.State(slug, phones)
}

func detailPhones(detail ikman.Detail) []string {
	var phones []string
	for _, phone := range detail.Ad.ContactCard.PhoneNumbers {
		if phone.Number != "" {
			phones = append(phones, phone.Number)
		}
	}
	if len(phones) == 0 {
		phones = calls.SplitPhones(detail.Ad.PhoneText())
	}
	return phones
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func pageURL(r *http.Request, page int) string {
	if page < 1 {
		return ""
	}
	values := r.URL.Query()
	values.Set("page", strconv.Itoa(page))
	return "/ads?" + values.Encode()
}

func listReturnURL(r *http.Request) string {
	values := r.URL.Query()
	values.Del("back")
	if encoded := values.Encode(); encoded != "" {
		return "/ads?" + encoded
	}
	return "/ads"
}

func nextAdsURL(r *http.Request, page int) string {
	if page < 1 {
		return ""
	}
	values := r.URL.Query()
	values.Set("page", strconv.Itoa(page))
	return "/api/ads?" + values.Encode()
}

func detailBackURL(r *http.Request) string {
	if backURL := sanitizeBackURL(r.URL.Query().Get("back")); backURL != "" {
		return backURL
	}
	values := r.URL.Query()
	values.Del("back")
	if encoded := values.Encode(); encoded != "" {
		return "/ads?" + encoded
	}
	return "/ads"
}

func sanitizeBackURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return ""
	}
	if parsed.Path == "" {
		parsed.Path = "/ads"
	}
	if parsed.Path != "/ads" {
		return ""
	}
	parsed.Fragment = ""
	return parsed.RequestURI()
}

func detailURL(slug, backURL string) string {
	if strings.TrimSpace(slug) == "" {
		return "/ads"
	}
	path := "/ads/" + url.PathEscape(slug)
	if backURL = sanitizeBackURL(backURL); backURL != "" {
		path += "?back=" + url.QueryEscape(backURL)
	}
	return path
}

func queryBool(values url.Values, key string, fallback bool) bool {
	raw, ok := values[key]
	if !ok {
		return fallback
	}
	for _, value := range raw {
		if value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "on") {
			return true
		}
	}
	return false
}

func selected(current, value string) template.HTMLAttr {
	if current == value {
		return "selected"
	}
	return ""
}

func checked(value bool) template.HTMLAttr {
	if value {
		return "checked"
	}
	return ""
}

func phoneCSS(phone string) string {
	if phone == "" || phone == "Unavailable" {
		return "muted"
	}
	return "phone"
}

func nl2br(value string) template.HTML {
	escaped := template.HTMLEscapeString(value)
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

func categoryOptions() []Option {
	return []Option{
		{"All categories", ""},
		{"Vehicles", "vehicles"},
		{"Cars", "cars"},
		{"Motorbikes", "motorbikes-scooters"},
		{"Auto parts", "auto-parts-accessories"},
		{"Property", "property"},
		{"Land", "land-for-sale"},
		{"Houses for sale", "houses-for-sale"},
		{"House rentals", "house-rentals"},
		{"Apartments for sale", "apartments-for-sale"},
		{"Mobiles", "mobiles"},
		{"Mobile phones", "mobile-phones"},
		{"Electronics", "electronics"},
		{"Computers", "computers-tablets"},
		{"TVs", "tvs"},
		{"Air conditioners", "air-conditions-electrical-fittings"},
		{"Home & garden", "home-garden"},
		{"Furniture", "furniture"},
		{"Jobs", "jobs"},
		{"Services", "services"},
		{"Pets", "pets"},
		{"Fashion & beauty", "fashion-beauty"},
	}
}

func locationOptions() []Option {
	return []Option{
		{"All Sri Lanka", ""},
		{"Colombo", "colombo"},
		{"Gampaha", "gampaha"},
		{"Kandy", "kandy"},
		{"Kalutara", "kalutara"},
		{"Galle", "galle"},
		{"Matara", "matara"},
		{"Kurunegala", "kurunegala"},
		{"Puttalam", "puttalam"},
		{"Anuradhapura", "anuradhapura"},
		{"Jaffna", "jaffna"},
		{"Batticaloa", "batticaloa"},
		{"Badulla", "badulla"},
		{"Ratnapura", "ratnapura"},
		{"Matale", "matale"},
	}
}

func adTypeOptions() []Option {
	return []Option{
		{"Any type", ""},
		{"For sale", "for_sale"},
		{"For rent", "for_rent"},
		{"Wanted", "to_buy"},
		{"Jobs", "job"},
	}
}

func sortOptions() []Option {
	return []Option{
		{"ikman order", ""},
		{"Price low to high", "price_asc"},
		{"Price high to low", "price_desc"},
		{"Title A-Z", "title_asc"},
		{"Most photos", "images_desc"},
		{"Location A-Z", "location_asc"},
		{"Category A-Z", "category_asc"},
	}
}

func calledOptions() []Option {
	return []Option{
		{"Show all", ""},
		{"Hide called", "hide"},
		{"Only called", "only"},
	}
}
