/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package geregecore reads the Gerege Core directory: the register of
// organisations and people this ecosystem already holds.
//
// It is here so that standing a deployment up does not mean typing in an
// organisation somebody has already registered. A registration number is
// enough to fill the name, the legal name and the contact details; a person's
// registration number is enough to fill their name and address. Typing them
// again is how a deployment ends up with a name that does not match the one on
// the invoice.
//
// Read-only, and deliberately narrow: two lookups, no writes, no caching. A
// directory this platform does not own is not a directory it should be holding
// a stale copy of.
//
// In the kernel rather than in either plane because both reach it: the console
// plane stands a deployment up from a registration number, and the tenant plane
// refreshes an organisation's own details from the same record. Neither plane
// imports the other, so a client used by both is the floor underneath them.
package geregecore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// DefaultBaseURL is the production directory. Overridden by GEREGE_CORE_URL for
// a staging one.
const DefaultBaseURL = "https://core.gerege.mn"

// Client talks to the directory. The zero value is not usable; see New.
type Client struct {
	baseURL string
	token   func() string
	http    *http.Client
}

// New builds a client. An empty baseURL means the production directory.
//
// token is a function rather than a string because the token is a console
// credential: an operator can set or rotate it while the process is running,
// and a client that captured the value at construction would go on presenting
// the old one — or none — until somebody restarted the deployment. An empty
// answer means the directory is not configured, which Configured reports and
// every lookup refuses rather than calling an endpoint that would 401.
func New(baseURL string, token func() string) *Client {
	if baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/"); baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if token == nil {
		token = func() string { return "" }
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		// Long enough for a directory that reaches a government register
		// behind it, short enough that a wizard does not look hung.
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// Configured reports whether a token was supplied. Without one the wizard shows
// the fields to fill in by hand rather than a lookup that cannot work.
func (c *Client) Configured() bool { return c != nil && strings.TrimSpace(c.token()) != "" }

// ErrNotConfigured is a lookup on a deployment with no directory token.
var ErrNotConfigured = fmt.Errorf("GEREGE_CORE_TOKEN is not set, so the Gerege Core directory cannot be searched")

// ErrNotFound is a registration number the directory does not know.
var ErrNotFound = fmt.Errorf("the Gerege Core directory has no record with that number")

// Organisation is what the directory holds about a registered organisation.
// The field names are the directory's own, so that what arrives and what is
// read are the same words.
type Organisation struct {
	ID            int64  `json:"id"`
	RegNo         string `json:"reg_no"`
	Name          string `json:"name"`
	ShortName     string `json:"short_name"`
	PhoneNo       string `json:"phone_no"`
	Email         string `json:"email"`
	LogoImageURL  string `json:"logo_image_url"`
	AddressDetail string `json:"address_detail"`
	CountryCode   string `json:"country_code"`
	CountryName   string `json:"country_name"`
}

// Person is what it holds about a registered person. Only the parts a
// deployment's first administrator is made of are kept: the directory returns
// an address and a civil identifier as well, and neither belongs in a row this
// platform did not need to create.
type Person struct {
	ID         int64  `json:"id"`
	RegNo      string `json:"reg_no"`
	FamilyName string `json:"family_name"`
	LastName   string `json:"last_name"`
	FirstName  string `json:"first_name"`
	Email      string `json:"email"`
	PhoneNo    string `json:"phone_no"`
}

// FullName is the person's name in the order Mongolian uses: family name then
// given name. The directory stores both in lower case, so each word is
// capitalised — a name rendered "эрдэнэбат" is a name somebody will retype.
func (p Person) FullName() string {
	parts := make([]string, 0, 2)
	for _, part := range []string{p.LastName, p.FirstName} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, capitalise(part))
		}
	}
	return strings.Join(parts, " ")
}

func capitalise(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if i == 0 || runes[i-1] == ' ' || runes[i-1] == '-' {
			runes[i] = unicode.ToUpper(r)
		}
	}
	return string(runes)
}

// FindOrganisation looks an organisation up by its registration number.
func (c *Client) FindOrganisation(ctx context.Context, regNo string) (Organisation, error) {
	var org Organisation
	if !c.Configured() {
		return org, ErrNotConfigured
	}
	if regNo = strings.TrimSpace(regNo); regNo == "" {
		return org, fmt.Errorf("a registration number is required")
	}
	endpoint := c.baseURL + "/api/organization/find?search_text=" + url.QueryEscape(regNo)
	if err := c.call(ctx, http.MethodGet, endpoint, nil, &org); err != nil {
		return Organisation{}, err
	}
	if org.RegNo == "" && org.Name == "" {
		return Organisation{}, ErrNotFound
	}
	return org, nil
}

// FindPerson looks a person up by their registration number or their Core id.
//
// countryCode is what the directory asks for and defaults to Mongolia: the
// endpoint refuses the request without it, and a wizard standing up a
// deployment in Mongolia should not have to know that.
func (c *Client) FindPerson(ctx context.Context, search, countryCode string) (Person, error) {
	var person Person
	if !c.Configured() {
		return person, ErrNotConfigured
	}
	if search = strings.TrimSpace(search); search == "" {
		return person, fmt.Errorf("a registration number is required")
	}
	if countryCode = strings.TrimSpace(countryCode); countryCode == "" {
		countryCode = "MN"
	}
	body, err := json.Marshal(map[string]string{"search_text": search, "country_code": countryCode})
	if err != nil {
		return person, err
	}
	if err := c.call(ctx, http.MethodPost, c.baseURL+"/api/user/find", body, &person); err != nil {
		return Person{}, err
	}
	if person.RegNo == "" && person.Email == "" {
		return Person{}, ErrNotFound
	}
	return person, nil
}

// call performs one request and decodes it.
//
// The directory answers a miss with 200 and {"message": "record not found"}
// rather than with a status, so the body is read before the status is trusted:
// a decoder that only looked at the code would hand back an empty record and
// call it a hit.
// notFoundPhrases are how the directory says it has no such record.
//
// It says it in Mongolian, and it says it with a 500: a search for a
// registration number nobody is registered under answers
//
//	HTTP 500 {"message":"Мэдээлэл олдсонгүй"}
//
// The English phrase was the only one recognised until this list, so the
// commonest outcome of a mistyped number — there is no such organisation —
// reached the wizard as `502 the directory refused: Мэдээлэл олдсонгүй`
// instead of the 404 the screen has a sentence for. Both are kept: the
// directory is somebody else's service and its wording is theirs to change.
var notFoundPhrases = []string{"not found", "олдсонгүй", "бүртгэлгүй"}

// saysNotFound reports whether the directory's message means "no such record".
func saysNotFound(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	for _, phrase := range notFoundPhrases {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}

func (c *Client) call(ctx context.Context, method, endpoint string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.token()))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("the Gerege Core directory could not be reached: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("the directory's answer could not be read: %w", err)
	}

	var problem struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &problem); err == nil && problem.Message != "" {
		if saysNotFound(problem.Message) {
			return ErrNotFound
		}
		return fmt.Errorf("the directory refused: %s", problem.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the directory answered %d", resp.StatusCode)
	}
	return json.Unmarshal(payload, out)
}
