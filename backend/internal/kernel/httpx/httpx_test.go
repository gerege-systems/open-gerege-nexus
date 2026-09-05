package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeLimited(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		max     int64
		wantErr bool
	}{
		{"exact limit", `{"name":"ok"}`, 13, false},
		{"trailing whitespace", "{\"name\":\"ok\"} \n", 16, false},
		{"oversized value", `{"name":"too long"}`, 13, true},
		{"oversized whitespace", `{"name":"ok"}` + strings.Repeat(" ", 20), 13, true},
		{"second value", `{"name":"ok"}{}`, 64, true},
		{"trailing garbage", `{"name":"ok"}garbage`, 64, true},
		{"empty", "", 64, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			var dst struct {
				Name string `json:"name"`
			}
			err := DecodeLimited(r, &dst, tt.max)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeLimited error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
