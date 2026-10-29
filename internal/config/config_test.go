package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	a, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.IdleTimeoutSeconds != 60 || a.Mode != "mouse" {
		t.Fatalf("bad default: %+v", a)
	}
	a.MovementPixels = 2
	if err = Save(p, a); err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil || b != a {
		t.Fatalf("roundtrip: %+v %v", b, err)
	}
}
func TestInvalid(t *testing.T) {
	c := Default()
	c.Mode = "invalid"
	if c.Validate() == nil {
		t.Fatal("accepted mode")
	}
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("accepted broken JSON")
	}
}

func TestUTF8BOM(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := Default()
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append([]byte{0xef, 0xbb, 0xbf}, b...), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || got != c {
		t.Fatalf("BOM: %+v %v", got, err)
	}
}
