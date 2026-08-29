/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package geregecore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The directory says "no such record" in Mongolian, and with a 500.
//
// Observed against the live directory on 2026-08-29: a search for a
// registration number nobody is registered under answers
// `HTTP 500 {"message":"Мэдээлэл олдсонгүй"}`. Only the English phrase was
// recognised, so the commonest outcome of a mistyped number reached the wizard
// as a bad gateway rather than as the not-found the screen has a sentence for.
func TestTheDirectorysMongolianNotFoundIsANotFound(t *testing.T) {
	for _, message := range []string{
		"Мэдээлэл олдсонгүй",
		"мэдээлэл олдсонгүй",
		"Байгууллага олдсонгүй",
		"record not found",
		"Not Found",
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"` + message + `"}`))
		}))
		client := New(server.URL, func() string { return "a-token" })
		_, err := client.FindOrganisation(context.Background(), "1234567")
		server.Close()
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%q was not read as a missing record: %v", message, err)
		}
	}
}

// A refusal that is not a missing record keeps its own words: the operator
// reading it needs to know the directory said no, and why.
func TestAnotherRefusalIsNotTurnedIntoANotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Токен хүчингүй"}`))
	}))
	defer server.Close()

	client := New(server.URL, func() string { return "a-token" })
	_, err := client.FindOrganisation(context.Background(), "1234567")
	if errors.Is(err, ErrNotFound) {
		t.Fatal("an invalid token was reported as a missing record")
	}
	if err == nil {
		t.Fatal("a refusal was not reported at all")
	}
}

// An organisation that is found still comes back.
func TestAFoundOrganisationIsReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer a-token" {
			t.Errorf("the token was not presented: %q", got)
		}
		_, _ = w.Write([]byte(`{"reg_no":"1234567","name":"Герэгэ Системс"}`))
	}))
	defer server.Close()

	client := New(server.URL, func() string { return "a-token" })
	org, err := client.FindOrganisation(context.Background(), "1234567")
	if err != nil {
		t.Fatalf("a found organisation was refused: %v", err)
	}
	if org.Name != "Герэгэ Системс" {
		t.Errorf("the answer was not read: %+v", org)
	}
}
