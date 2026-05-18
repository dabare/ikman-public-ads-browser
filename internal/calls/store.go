package calls

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Store struct {
	path string
	mu   sync.RWMutex
	db   database
}

type database struct {
	Version int                     `json:"version"`
	Ads     map[string]AdRecord     `json:"ads"`
	Numbers map[string]NumberRecord `json:"numbers"`
}

type AdRecord struct {
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	Phones   []string  `json:"phones"`
	CalledAt time.Time `json:"called_at"`
}

type NumberRecord struct {
	Number       string    `json:"number"`
	DisplayPhone string    `json:"display_phone"`
	Slugs        []string  `json:"slugs"`
	LastCalledAt time.Time `json:"last_called_at"`
}

type State struct {
	Called              bool     `json:"called"`
	CalledBefore        bool     `json:"calledBefore"`
	CalledBeforeNumbers []string `json:"calledBeforeNumbers"`
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("missing call database path")
	}
	store := &Store{path: path, db: database{Version: 1, Ads: map[string]AdRecord{}, Numbers: map[string]NumberRecord{}}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err == nil {
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &store.db); err != nil {
				return nil, err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	store.repairLocked()
	if err := store.saveLocked(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Mark(slug, title string, phones []string, called bool) (State, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return State{}, errors.New("missing ad slug")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !called {
		delete(s.db.Ads, slug)
		s.rebuildNumbersLocked()
		return s.stateLocked(slug, phones), s.saveLocked()
	}

	now := time.Now().UTC()
	if existing, ok := s.db.Ads[slug]; ok && !existing.CalledAt.IsZero() {
		now = existing.CalledAt
	}
	record := AdRecord{
		Slug:     slug,
		Title:    strings.TrimSpace(title),
		Phones:   normalizePhoneList(phones),
		CalledAt: now,
	}
	s.db.Ads[slug] = record
	s.rebuildNumbersLocked()
	return s.stateLocked(slug, phones), s.saveLocked()
}

func (s *Store) State(slug string, phones []string) State {
	if s == nil {
		return State{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateLocked(slug, phones)
}

func (s *Store) stateLocked(slug string, phones []string) State {
	_, called := s.db.Ads[strings.TrimSpace(slug)]
	state := State{Called: called}
	seen := map[string]bool{}
	for _, phone := range normalizePhoneList(phones) {
		number := NormalizePhone(phone)
		record, ok := s.db.Numbers[number]
		if !ok {
			continue
		}
		if !hasOtherSlug(record.Slugs, slug) {
			continue
		}
		display := record.DisplayPhone
		if display == "" {
			display = phone
		}
		if !seen[display] {
			seen[display] = true
			state.CalledBeforeNumbers = append(state.CalledBeforeNumbers, display)
		}
	}
	sort.Strings(state.CalledBeforeNumbers)
	state.CalledBefore = len(state.CalledBeforeNumbers) > 0
	return state
}

func (s *Store) repairLocked() {
	if s.db.Version == 0 {
		s.db.Version = 1
	}
	if s.db.Ads == nil {
		s.db.Ads = map[string]AdRecord{}
	}
	if s.db.Numbers == nil {
		s.db.Numbers = map[string]NumberRecord{}
	}
	s.rebuildNumbersLocked()
}

func (s *Store) rebuildNumbersLocked() {
	numbers := map[string]NumberRecord{}
	for slug, ad := range s.db.Ads {
		for _, phone := range normalizePhoneList(ad.Phones) {
			number := NormalizePhone(phone)
			if number == "" {
				continue
			}
			record := numbers[number]
			record.Number = number
			if record.DisplayPhone == "" {
				record.DisplayPhone = phone
			}
			if !contains(record.Slugs, slug) {
				record.Slugs = append(record.Slugs, slug)
			}
			if ad.CalledAt.After(record.LastCalledAt) {
				record.LastCalledAt = ad.CalledAt
				record.DisplayPhone = phone
			}
			numbers[number] = record
		}
	}
	for number, record := range numbers {
		sort.Strings(record.Slugs)
		numbers[number] = record
	}
	s.db.Numbers = numbers
}

func (s *Store) saveLocked() error {
	body, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func NormalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}
	value := digits.String()
	if len(value) == 9 && strings.HasPrefix(value, "7") {
		return "0" + value
	}
	if len(value) == 11 && strings.HasPrefix(value, "94") {
		return "0" + value[2:]
	}
	return value
}

func SplitPhones(value string) []string {
	return normalizePhoneList([]string{value})
}

func normalizePhoneList(values []string) []string {
	seen := map[string]bool{}
	var phones []string
	for _, value := range values {
		for _, part := range splitPhoneFields(value) {
			number := NormalizePhone(part)
			if part == "" || number == "" || seen[number] {
				continue
			}
			seen[number] = true
			phones = append(phones, part)
		}
	}
	return phones
}

func splitPhoneFields(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '/' || r == '\n' || r == '\t'
	})
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func hasOtherSlug(slugs []string, slug string) bool {
	slug = strings.TrimSpace(slug)
	for _, value := range slugs {
		if value != "" && value != slug {
			return true
		}
	}
	return false
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
