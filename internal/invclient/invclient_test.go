package invclient

import (
	"context"
	"errors"
	"testing"
)

func TestFake(t *testing.T) {
	f := NewFake()
	f.Set("t", []Host{{ID: "h1", Hostname: "a"}})
	hs, err := f.ListHosts(context.Background(), "t")
	if err != nil || len(hs) != 1 || hs[0].Hostname != "a" {
		t.Fatal(hs, err)
	}
	if hs, _ := f.ListHosts(context.Background(), "other"); len(hs) != 0 {
		t.Fatal("tenant scoped")
	}
	f.Down = true
	if _, err := f.ListHosts(context.Background(), "t"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
