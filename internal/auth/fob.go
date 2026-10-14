package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/repository"
)

// FobWindowDuration is how long "Fob koppeln" waits for a fob at the reader.
// Long enough to walk to the table and press a button, short enough that a
// window nobody used does not keep the next person waiting (docs/adr/0025).
const FobWindowDuration = 60 * time.Second

// ErrUnknownFob is the answer for a fob nobody owns while no window is open.
// It carries nothing about the fob on purpose: whoever logs it logs nothing
// that identifies a card (docs/adr/0025).
var ErrUnknownFob = errors.New("unknown fob")

// ErrBadUID is a UID that is not what a reader produces: hex, 4, 7 or 10
// bytes. It is a bug on the sending side, not a card, so it says so.
var ErrBadUID = errors.New("a uid is 4, 7 or 10 bytes of hex")

// Fobs resolves key fobs to players for the Zählwerk and lets a player link
// and remove their own.
//
// It sits here, beside the cookie, because a fob is a provider (invariant 4):
// nothing outside this package knows that a fob is stored as an HMAC or which
// provider name it has. It is not an Authenticator, though. A UID can be read
// and copied by anybody standing close enough, so a fob names a player for a
// match that still has to be confirmed and never opens a session.
type Fobs struct {
	store repository.Store
	key   []byte
	now   func() time.Time
}

// NewFobs builds the resolver. key is SP_RFID_UID_KEY; the caller does not
// build a Fobs at all without one.
func NewFobs(store repository.Store, key []byte) *Fobs {
	return &Fobs{store: store, key: key, now: time.Now}
}

// WithClock replaces the clock. Tests only.
func (f *Fobs) WithClock(now func() time.Time) *Fobs {
	f.now = now
	return f
}

// FobCheckin is what a successful resolve hands the Zählwerk.
type FobCheckin struct {
	PlayerID  uuid.UUID
	CheckinID uuid.UUID
	// Linked is true when this resolve linked the fob, which the caller logs:
	// a new way in to somebody's account deserves a line.
	Linked bool
}

// Resolve names the player behind uid and records their check-in at half.
//
// A known fob resolves to its owner. An unknown one is linked to whoever holds
// the open window, and that also checks them in. An unknown one with no
// window is ErrUnknownFob, and nothing is written.
//
// A fob that already belongs to somebody is never relinked, and it does not
// use up somebody else's window.
func (f *Fobs) Resolve(ctx context.Context, uid string, half domain.Half) (FobCheckin, error) {
	subject, err := f.subject(uid)
	if err != nil {
		return FobCheckin{}, err
	}
	if !half.Valid() {
		return FobCheckin{}, fmt.Errorf("half %q: want left or right", half)
	}

	at := f.now()
	var out FobCheckin
	err = f.store.InTx(ctx, func(tx repository.Store) error {
		player, err := tx.Identities().PlayerBy(ctx, domain.ProviderRFID, subject)
		switch {
		case err == nil:
			out.PlayerID = player.ID
		case errors.Is(err, domain.ErrNotFound):
			window, err := tx.FobWindow().Consume(ctx, at)
			if errors.Is(err, domain.ErrNotFound) {
				return ErrUnknownFob
			}
			if err != nil {
				return err
			}
			if err := tx.Identities().Link(ctx, domain.ProviderRFID, subject, window.PlayerID); err != nil {
				return fmt.Errorf("link fob: %w", err)
			}
			out.PlayerID, out.Linked = window.PlayerID, true
		default:
			return fmt.Errorf("look up fob: %w", err)
		}

		if err := tx.Identities().Touch(ctx, domain.ProviderRFID, subject, at); err != nil {
			return fmt.Errorf("touch fob: %w", err)
		}
		checkin, err := tx.Checkins().Create(ctx, domain.Checkin{
			PlayerID:  out.PlayerID,
			Half:      half,
			Method:    domain.CheckinRFID,
			CreatedAt: at,
		})
		if err != nil {
			return err
		}
		out.CheckinID = checkin.ID
		return nil
	})
	if err != nil {
		return FobCheckin{}, err
	}
	return out, nil
}

// OpenWindow opens the link window for playerID. While somebody else holds
// one, the error is domain.ErrConflict and the window returned is theirs, so
// the page can say until when.
func (f *Fobs) OpenWindow(ctx context.Context, playerID uuid.UUID) (domain.FobWindow, error) {
	at := f.now()
	return f.store.FobWindow().Open(ctx, playerID, at, at.Add(FobWindowDuration))
}

// Window returns the open window, whoever holds it, or domain.ErrNotFound.
func (f *Fobs) Window(ctx context.Context) (domain.FobWindow, error) {
	return f.store.FobWindow().Current(ctx, f.now())
}

// CloseWindow gives playerID's window up early.
func (f *Fobs) CloseWindow(ctx context.Context, playerID uuid.UUID) error {
	return f.store.FobWindow().Close(ctx, playerID)
}

// ForPlayer lists playerID's fobs, oldest first.
func (f *Fobs) ForPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.Identity, error) {
	all, err := f.store.Identities().ForPlayer(ctx, playerID)
	if err != nil {
		return nil, err
	}
	var fobs []domain.Identity
	for _, id := range all {
		if id.Provider == domain.ProviderRFID {
			fobs = append(fobs, id)
		}
	}
	return fobs, nil
}

// Remove unlinks one of playerID's fobs. A subject that is not one of theirs
// is domain.ErrNotFound, so nobody removes somebody else's fob by guessing at
// a form value.
func (f *Fobs) Remove(ctx context.Context, playerID uuid.UUID, subject string) error {
	fobs, err := f.ForPlayer(ctx, playerID)
	if err != nil {
		return err
	}
	for _, fob := range fobs {
		if hmac.Equal([]byte(fob.Subject), []byte(subject)) {
			return f.store.Identities().Unlink(ctx, domain.ProviderRFID, subject)
		}
	}
	return fmt.Errorf("fob of player %s: %w", playerID, domain.ErrNotFound)
}

// subject is HMAC-SHA256(key, uid) over the normalised UID, hex-encoded.
//
// Keyed, because a plain hash over four to seven bytes is a table somebody
// computes in minutes, and then it is the UID again (docs/adr/0025).
func (f *Fobs) subject(uid string) (string, error) {
	normal, err := normaliseUID(uid)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, f.key)
	mac.Write([]byte("rfid:"))
	mac.Write(normal)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// normaliseUID accepts what readers and firmware print — "04:A3:1B:22",
// "04a31b22", "04 A3 1B 22" — and returns the bytes, so one fob is one
// subject however it was written down.
func normaliseUID(uid string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ':', '-', ' ':
			return -1
		}
		return r
	}, uid)
	raw, err := hex.DecodeString(clean)
	if err != nil {
		return nil, ErrBadUID
	}
	switch len(raw) {
	case 4, 7, 10:
		return raw, nil
	}
	return nil, ErrBadUID
}
