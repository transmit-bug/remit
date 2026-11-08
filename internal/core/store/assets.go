package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// An asset is a non-text file a note carries — today, an image. It lives in
// the library's assets/ directory named by its own content (ADR 0003), so the
// same picture dropped twice is stored once and a note references it by a
// relative path that survives a move or a copy of the library.

// assetsDir is the library's directory of assets.
const assetsDir = "assets"

// assetPrefix is how a note names an asset: a path relative to the library
// directory.
const assetPrefix = assetsDir + "/"

// PutAsset stores an asset by its content and returns the relative path a
// note references it by — "assets/<sha256>.<ext>". Storing the same bytes
// again names the same file and writes nothing, so a repeated drop or paste
// costs one file. ext is the name's extension without the dot ("png"); it
// stays as given, lower-cased, so the file opens by its type.
func (l *Library) PutAsset(data []byte, ext string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return "", ErrClosed
	}
	rel, err := assetName(data, ext)
	if err != nil {
		return "", err
	}
	path := filepath.Join(l.dir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		return rel, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(l.dir, assetsDir), dirPerms); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, data); err != nil {
		return "", err
	}
	return rel, nil
}

// Asset reads an asset's bytes by the relative path a note references it
// with. Only paths inside the library's own assets directory resolve: a
// reference that names another directory, climbs out with "..", or carries a
// scheme has no asset, so a note cannot make the library read a file outside
// it. The second answer is false when no such asset is stored.
func (l *Library) Asset(rel string) ([]byte, bool, error) {
	name, ok := assetFileName(rel)
	if !ok {
		return nil, false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.index == nil {
		return nil, false, ErrClosed
	}
	data, err := os.ReadFile(filepath.Join(l.dir, assetsDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// assetName is the relative path data is stored under.
func assetName(data []byte, ext string) (string, error) {
	ext = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
	switch ext {
	case "":
		return "", fmt.Errorf("asset: an asset needs an extension")
	case "jpg":
		// The picture formats the library can show are stored under one
		// spelling each, so the same picture from two apps lands as one
		// file.
		ext = "jpeg"
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%s/%s.%s", assetsDir, hex.EncodeToString(sum[:]), ext), nil
}

// assetFileName reads the file name of a reference a note carries, refusing
// everything that is not the library's own assets directory: a bare name, a
// climb out of it, another directory, an absolute path, a scheme.
func assetFileName(rel string) (string, bool) {
	if !strings.HasPrefix(rel, assetPrefix) {
		return "", false
	}
	name := rel[len(assetPrefix):]
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) ||
		strings.Contains(name, "..") || strings.HasPrefix(name, ".") {
		return "", false
	}
	return name, true
}
