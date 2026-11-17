package memory

import "testing"

func TestParseSize(t *testing.T) {
	got, err := ParseSize("512MB")
	if err != nil {
		t.Fatal(err)
	}
	if got != 512*1024*1024 {
		t.Fatalf("got %d", got)
	}
}
