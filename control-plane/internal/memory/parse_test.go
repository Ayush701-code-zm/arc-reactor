package memory

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"512MB", 512 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"1024KB", 1024 * 1024},
		{"100", 100},
	}
	for _, c := range cases {
		got, err := ParseSize(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%s: got %d want %d", c.in, got, c.want)
		}
	}
}

func TestParseSizeErrors(t *testing.T) {
	for _, in := range []string{"", "MB", "0MB", "1TB"} {
		if _, err := ParseSize(in); err == nil {
			t.Fatalf("%q: expected error", in)
		}
	}
}
