package store

import (
	"bytes"
	"testing"
)

// newTestLib opens a throwaway library for a test.
func newTestLib(t *testing.T) *Library {
	t.Helper()
	lib, err := Create(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib
}

func TestPutAssetStoresByNameContentAndIsIdempotent(t *testing.T) {
	lib := newTestLib(t)

	png := []byte("png-bytes-one")
	rel, err := lib.PutAsset(png, "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}
	if want := "assets/afe8ed7a3ec867b9c78c0cb490618fa61e970df3fa8c882ba5aeb783f304c856.png"; rel != want {
		t.Errorf("PutAsset rel = %q, want %q", rel, want)
	}

	// The same bytes again: the same name, nothing written twice.
	again, err := lib.PutAsset(png, "png")
	if err != nil {
		t.Fatalf("put asset again: %v", err)
	}
	if again != rel {
		t.Errorf("PutAsset again = %q, want %q", again, rel)
	}

	// A different spelling of the same format lands as one file.
	jpg, err := lib.PutAsset(png, "jpg")
	if err != nil {
		t.Fatalf("put jpg: %v", err)
	}
	if jpg != "assets/afe8ed7a3ec867b9c78c0cb490618fa61e970df3fa8c882ba5aeb783f304c856.jpeg" {
		t.Errorf("jpg asset = %q, want the jpeg spelling", jpg)
	}

	// The bytes round-trip.
	data, ok, err := lib.Asset(rel)
	if err != nil {
		t.Fatalf("asset: %v", err)
	}
	if !ok || !bytes.Equal(data, png) {
		t.Errorf("Asset = (%v, %q), want the stored bytes", ok, data)
	}
}

func TestAssetRefusesPathsOutsideTheAssetsDirectory(t *testing.T) {
	lib := newTestLib(t)

	for _, rel := range []string{
		"notes/hand-written.md",
		"remit.json",
		"assets/../remit.json",
		"assets/sub/dir.png",
		"assets/.hidden.png",
		"assets/",
		"/etc/passwd",
		"//example.com/x.png",
		"",
	} {
		data, ok, err := lib.Asset(rel)
		if err == nil && ok {
			t.Errorf("Asset(%q) returned data %q, want a refusal", rel, data)
		}
	}
}

func TestAssetAnswersFalseForAMissingFile(t *testing.T) {
	lib := newTestLib(t)
	rel, err := lib.PutAsset([]byte("real"), "png")
	if err != nil {
		t.Fatalf("put asset: %v", err)
	}

	if _, ok, err := lib.Asset("assets/0000000000000000000000000000000000000000000000000000000000000000.png"); err != nil || ok {
		t.Errorf("missing asset = (_, %v, %v), want (_, false, nil)", ok, err)
	}
	if _, ok, err := lib.Asset(rel); err != nil || !ok {
		t.Errorf("stored asset = (_, %v, %v), want (_, true, nil)", ok, err)
	}
}
