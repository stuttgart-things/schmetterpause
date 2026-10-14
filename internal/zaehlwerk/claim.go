// Package zaehlwerk is the one call this application makes to the Zählwerk:
// claiming a table half for a player who checked in with their phone
// (docs/adr/0026).
//
// Every other exchange runs the other way, the Zählwerk calling /api here.
// This one cannot: the phone is signed in here, so only this application
// knows who is standing at the half, and the Zählwerk has to be told.
//
// No database, no templates. The server decides what to tell the player; this
// package only says what the Zählwerk answered.
package zaehlwerk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// claimTimeout bounds the call. A player is holding a phone and waiting for
// the page to answer; a few seconds is all the patience there is, and there
// is no retry behind it (docs/adr/0026).
const claimTimeout = 5 * time.Second

// maxAnswer bounds what is read back. The answer is a few names and ids.
const maxAnswer = 16 << 10

// ErrUnavailable is a Zählwerk that could not be asked or did not answer
// in a way this package understands. The player is told to try again.
var ErrUnavailable = errors.New("the Zählwerk is not available")

// Outcome is what the Zählwerk did with a claim, in its own words
// (zaehlwerk ADR-0010).
type Outcome string

// The outcomes a claim can have. They are the Zählwerk's, shared with its fob
// path, so both read alike.
const (
	Claimed   Outcome = "claimed"
	Unchanged Outcome = "unchanged"
	Replaced  Outcome = "replaced"
	Moved     Outcome = "moved"
	Started   Outcome = "started"
	// Refused is a claim while a match holds the table.
	Refused Outcome = "refused"
)

// Answer is the Zählwerk's reply to a claim.
type Answer struct {
	Outcome Outcome `json:"outcome"`
	// Left and Right are who holds each half now, by name.
	Left  string `json:"left"`
	Right string `json:"right"`
}

// Client claims halves on one Zählwerk.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New builds a client for the Zählwerk at base, sending token as a bearer.
func New(base, token string) *Client {
	return &Client{base: base, token: token, http: &http.Client{Timeout: claimTimeout}}
}

// claimRequest is the body of the Zählwerk's POST /checkin/claim.
type claimRequest struct {
	Half      domain.Half `json:"half"`
	PlayerID  uuid.UUID   `json:"player_id"`
	CheckinID uuid.UUID   `json:"checkin_id"`
}

// Claim tells the Zählwerk that playerID, with checkinID, holds half.
//
// A claim the Zählwerk took returns its Answer. A table that is busy returns
// an Answer with Outcome Refused and no error, because that is an answer the
// player gets, not a fault. Anything else is ErrUnavailable, wrapped with
// what went wrong for the log.
func (c *Client) Claim(ctx context.Context, half domain.Half, playerID, checkinID uuid.UUID) (Answer, error) {
	body, err := json.Marshal(claimRequest{Half: half, PlayerID: playerID, CheckinID: checkinID})
	if err != nil {
		return Answer{}, fmt.Errorf("encode the claim: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/checkin/claim", bytes.NewReader(body))
	if err != nil {
		return Answer{}, fmt.Errorf("build the claim: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return Answer{}, fmt.Errorf("claim %s: %w: %w", half, ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var answer Answer
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxAnswer)).Decode(&answer)

	switch {
	case resp.StatusCode == http.StatusOK && decodeErr == nil:
		return answer, nil
	case resp.StatusCode == http.StatusConflict:
		answer.Outcome = Refused
		return answer, nil
	}
	// The status is enough for the log. The body may name players, and a
	// log line about a failed claim has no use for them.
	return Answer{}, fmt.Errorf("claim %s: %w: status %d", half, ErrUnavailable, resp.StatusCode)
}
