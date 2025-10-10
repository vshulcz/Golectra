package misc

import (
	"encoding/hex"
	"testing"
)

func TestSumSHA256(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		key   string
		want  string
	}{
		{"empty both", []byte{}, "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello/world", []byte("hello"), "world", "936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"},
		{"bytes/key", []byte{0x00, 0x01, 0x02}, "key", "acc3dc23298dcb1aec9b764fbc38f8eaea64040c6cd2c0a7bc958be0cf06b292"},
		{"unicode", []byte("Привет"), "ключ", "68dbb03b4b69fd44385c59eb1b5386ec0b91e225f3fc1e03f1ec6841c53490ec"},
		{"nil value", nil, "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SumSHA256(tc.value, tc.key)
			if got != tc.want {
				t.Fatalf("SumSHA256(%v, %q) = %s; want %s", tc.value, tc.key, got, tc.want)
			}
		})
	}
}

func TestSumSHA256_Prop(t *testing.T) {
	value := []byte("samevalue")
	key := "k1"
	got1 := SumSHA256(value, key)
	got2 := SumSHA256(value, key)
	if got1 != got2 {
		t.Fatalf("SumSHA256 not deterministic: %s != %s", got1, got2)
	}

	other := SumSHA256(value, "k2")
	if got1 == other {
		t.Fatalf("different keys produced same sum: %s == %s", got1, other)
	}

	decoded, err := hex.DecodeString(got1)
	if err != nil {
		t.Fatalf("result is not valid hex: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded length = %d, want 32", len(decoded))
	}
}
