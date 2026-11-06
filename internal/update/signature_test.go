package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func keyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return pub, priv, base64.StdEncoding.EncodeToString(pub)
}

func TestSignedArchiveVerifies(t *testing.T) {
	_, priv, pubB64 := keyPair(t)
	archive := []byte("the update archive")

	sig, err := SignArchive(priv, bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("SignArchive: %v", err)
	}
	if err := VerifyArchive(pubB64, sig, bytes.NewReader(archive)); err != nil {
		t.Fatalf("VerifyArchive: %v", err)
	}
}

func TestTamperedArchiveIsRejected(t *testing.T) {
	_, priv, pubB64 := keyPair(t)
	archive := []byte("the update archive")
	sig, err := SignArchive(priv, bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("SignArchive: %v", err)
	}

	tampered := append([]byte(nil), archive...)
	tampered[0] ^= 0xff
	if err := VerifyArchive(pubB64, sig, bytes.NewReader(tampered)); err == nil {
		t.Error("a tampered archive verified")
	}
}

func TestAnotherKeyIsRejected(t *testing.T) {
	_, priv, _ := keyPair(t)
	_, _, otherPub := keyPair(t)
	archive := []byte("the update archive")

	sig, err := SignArchive(priv, bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("SignArchive: %v", err)
	}
	if err := VerifyArchive(otherPub, sig, bytes.NewReader(archive)); err == nil {
		t.Error("a signature from another key verified")
	}
}

func TestMalformedInputs(t *testing.T) {
	_, priv, pub := keyPair(t)
	sig, err := SignArchive(priv, bytes.NewReader([]byte("x")))
	if err != nil {
		t.Fatalf("SignArchive: %v", err)
	}

	if err := VerifyArchive("not-base64", sig, bytes.NewReader([]byte("x"))); err == nil {
		t.Error("a malformed key verified")
	}
	if err := VerifyArchive(pub, "not-base64", bytes.NewReader([]byte("x"))); err == nil {
		t.Error("a malformed signature verified")
	}
}
