package zaehlwerk_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/zaehlwerk"
)

func TestClaimSendsTheContract(t *testing.T) {
	player, checkin := uuid.New(), uuid.New()
	var got struct {
		Half      string    `json:"half"`
		PlayerID  uuid.UUID `json:"player_id"`
		CheckinID uuid.UUID `json:"checkin_id"`
	}
	zw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/checkin/claim" {
			t.Errorf("request = %s %s, want POST /checkin/claim", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Errorf("Authorization = %q, want the bearer", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding the claim: %v", err)
		}
		_, _ = w.Write([]byte(`{"outcome":"started","half":"right","left":"Anna","right":"Bea","match_id":"m1"}`))
	}))
	defer zw.Close()

	answer, err := zaehlwerk.New(zw.URL, "secret").Claim(t.Context(), domain.HalfRight, player, checkin)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if got.Half != "right" || got.PlayerID != player || got.CheckinID != checkin {
		t.Errorf("sent %+v, want right, %s, %s", got, player, checkin)
	}
	if answer.Outcome != zaehlwerk.Started || answer.Left != "Anna" || answer.Right != "Bea" {
		t.Errorf("answer = %+v", answer)
	}
}

func TestClaimAnswers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		outcome zaehlwerk.Outcome
		err     bool
	}{
		{"busy table is an answer, not an error", http.StatusConflict,
			`{"outcome":"refused","half":"left","error":"a match is running"}`, zaehlwerk.Refused, false},
		{"busy table without a body", http.StatusConflict, ``, zaehlwerk.Refused, false},
		{"check-in off", http.StatusServiceUnavailable, `{"outcome":"unavailable"}`, "", true},
		{"wrong token", http.StatusUnauthorized, `{"error":"no"}`, "", true},
		{"route missing on an old Zählwerk", http.StatusNotFound, `404 page not found`, "", true},
		{"200 that is not JSON", http.StatusOK, `<html>`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer zw.Close()

			answer, err := zaehlwerk.New(zw.URL, "t").Claim(t.Context(), domain.HalfLeft, uuid.New(), uuid.New())
			if tc.err {
				if !errors.Is(err, zaehlwerk.ErrUnavailable) {
					t.Errorf("err = %v, want ErrUnavailable", err)
				}
				return
			}
			if err != nil || answer.Outcome != tc.outcome {
				t.Errorf("Claim = %+v, %v; want %q", answer, err, tc.outcome)
			}
		})
	}
}

func TestClaimWithNobodyListening(t *testing.T) {
	zw := httptest.NewServer(http.NotFoundHandler())
	url := zw.URL
	zw.Close()

	if _, err := zaehlwerk.New(url, "t").Claim(t.Context(), domain.HalfLeft, uuid.New(), uuid.New()); !errors.Is(err, zaehlwerk.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}
