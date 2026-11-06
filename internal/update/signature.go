package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// VerifyArchive checks an update archive against the app's public key.
//
// The scheme is MyGo's: the signature is Ed25519 over the archive's SHA-256
// digest, base64-encoded. Verification is independent of the operating
// system's code signing, so an unsigned build still verifies its updates
// (docs/release/signing.md).
func VerifyArchive(publicKey, signature string, archive io.Reader) error {
	key, err := decodePublicKey(publicKey)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("update: the signature is malformed")
	}
	digest, err := digestOf(archive)
	if err != nil {
		return fmt.Errorf("update: read archive: %w", err)
	}
	if !ed25519.Verify(key, digest, sig) {
		return errors.New("update: the archive is not signed with the app's key")
	}
	return nil
}

// SignArchive produces the signature VerifyArchive expects. It exists for
// tests and for a release pipeline that signs outside MyGo's own build; the
// private key never lives in the repository.
func SignArchive(privateKey ed25519.PrivateKey, archive io.Reader) (string, error) {
	digest, err := digestOf(archive)
	if err != nil {
		return "", fmt.Errorf("update: read archive: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, digest)), nil
}

func digestOf(r io.Reader) ([]byte, error) {
	sum := sha256.New()
	if _, err := io.Copy(sum, r); err != nil {
		return nil, err
	}
	return sum.Sum(nil), nil
}

func decodePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, errors.New("update: the public key is malformed")
	}
	return ed25519.PublicKey(b), nil
}
