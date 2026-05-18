package calls

import (
	"path/filepath"
	"testing"
)

func TestStoreTracksCalledAdsAndNumbers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "calls.json"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if _, err := store.Mark("old-ad", "Old ad", []string{"077 000 0000"}, true); err != nil {
		t.Fatalf("Mark(old-ad) error = %v", err)
	}
	state := store.State("new-ad", []string{"+94 77 0000000"})
	if !state.CalledBefore {
		t.Fatalf("CalledBefore = false, want true")
	}
	if len(state.CalledBeforeNumbers) != 1 || state.CalledBeforeNumbers[0] == "" {
		t.Fatalf("CalledBeforeNumbers = %#v, want saved number", state.CalledBeforeNumbers)
	}

	sameAd := store.State("old-ad", []string{"0770000000"})
	if !sameAd.Called {
		t.Fatalf("Called = false, want true")
	}
	if sameAd.CalledBefore {
		t.Fatalf("CalledBefore for same ad = true, want false")
	}
}

func TestStoreUnmarkRemovesNumberIndex(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "calls.json"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := store.Mark("old-ad", "Old ad", []string{"0770000000"}, true); err != nil {
		t.Fatalf("Mark(true) error = %v", err)
	}
	if _, err := store.Mark("old-ad", "", nil, false); err != nil {
		t.Fatalf("Mark(false) error = %v", err)
	}
	if state := store.State("new-ad", []string{"0770000000"}); state.CalledBefore {
		t.Fatalf("CalledBefore after unmark = true, want false")
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := map[string]string{
		"077 000 0000":    "0770000000",
		"+94 77 0000000":  "0770000000",
		"77 000 0000":     "0770000000",
		"011 234 5678":    "0112345678",
		"not a telephone": "",
	}
	for input, want := range tests {
		if got := NormalizePhone(input); got != want {
			t.Fatalf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}
