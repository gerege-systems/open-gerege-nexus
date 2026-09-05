package eidrp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestSessionRejectsIncompleteResponse(t *testing.T) {
	const body = `{"state":"RUNNING"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)+10))
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	c := NewClient(server.URL, "", "", "", "")
	result, err := c.Session(context.Background(), "s-1", 0)
	if !errors.Is(err, io.ErrUnexpectedEOF) || result != nil {
		t.Fatalf("incomplete response: result = %v, error = %v", result, err)
	}
}

func TestSessionResponseSizeLimit(t *testing.T) {
	const body = `{"state":"RUNNING"}`
	for _, size := range []int{maxRespBytes, maxRespBytes + 1} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body+strings.Repeat(" ", size-len(body)))
			}))
			defer server.Close()
			c := NewClient(server.URL, "", "", "", "")
			result, err := c.Session(context.Background(), "s-1", 0)
			if size > maxRespBytes {
				if err == nil || result != nil {
					t.Fatalf("oversized response accepted: %v, %v", result, err)
				}
			} else if err != nil || result.State != StateRunning {
				t.Fatalf("response at limit rejected: %v, %v", result, err)
			}
		})
	}
}
