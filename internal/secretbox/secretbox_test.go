package secretbox

import (
	"bytes"
	"testing"
)

func TestSealedValuesOpenOnlyWithTheirAssociatedData(t *testing.T) {
	box, err := New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal([]byte("token"), []byte("channel-a:access"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("token")) {
		t.Fatal("the plaintext is visible")
	}
	if opened, err := box.Open(sealed, []byte("channel-a:access")); err != nil || string(opened) != "token" {
		t.Fatalf("Open = %q, %v", opened, err)
	}
	if _, err := box.Open(sealed, []byte("channel-b:access")); err == nil {
		t.Fatal("a value moved to another channel must not open")
	}
	again, _ := box.Seal([]byte("token"), []byte("channel-a:access"))
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of the same value must differ")
	}
}
