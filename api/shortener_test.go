package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFollowChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(shortme))
	defer srv.Close()

	final, hops, err := follow(context.Background(), newFollowClient(), srv.URL+"/shortme?case=chain&left=3")
	if err != nil {
		t.Fatal(err)
	}
	if hops != 3 {
		t.Errorf("hops = %d, esperado 3", hops)
	}
	if !strings.Contains(final, "left=0") {
		t.Errorf("url final = %s, esperado o último hop (left=0)", final)
	}
}

func TestShortmeRejectsUnknownCase(t *testing.T) {
	rec := httptest.NewRecorder()
	shortme(rec, httptest.NewRequest(http.MethodGet, "/shortme?case=x", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado 400", rec.Code)
	}
}
