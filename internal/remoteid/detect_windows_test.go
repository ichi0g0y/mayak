//go:build windows

package remoteid

import "testing"

func TestDecodeValue(t *testing.T) {
	if got := decodeValue(append([]byte{0}, []byte(`"AB12"`)...)); got != "AB12" {
		t.Fatalf("decodeValue() = %q", got)
	}
}
