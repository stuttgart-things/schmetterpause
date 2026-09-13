package templates

import (
	"crypto/rand"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/stuttgart-things/schmetterpause/internal/credential"
)

// RecoveryFileName is what the saved code is called. It names the app and the
// thing, because the moment somebody needs it is weeks later, in a downloads
// folder, looking for "that file from the table tennis app".
const RecoveryFileName = "schmetterpause-wiederherstellungscode.txt"

// PINExample is a PIN to show beside a field that chooses one.
//
// Drawn fresh on every call rather than written into the template: an example
// next to an empty field gets copied, and a fixed one would become the most
// common PIN in the office and everybody's first guess (docs/adr/0018).
//
// crypto/rand for the same reason, although nothing depends on the example
// staying secret: a predictable sequence would be the fixed example again,
// only spread over a few values. The slight bias of a byte modulo ten does
// not matter for a number nobody is meant to use.
func PINExample() string {
	var b [credential.MinPINLength]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Same trade as credential.NewCode: no usable entropy source is a
		// broken process, not a page to render around.
		panic("crypto/rand unavailable: " + err.Error())
	}
	for i, v := range b {
		b[i] = '0' + v%10
	}
	return string(b[:])
}

// recoveryFileURL is the recovery code as a text file, in a data: URL.
//
// Built from the code already in the response and from nothing else, so there
// is no endpoint that could hand a code out again (docs/adr/0018). The text
// says what the code is for, because the file is read long after the card
// that explained it is gone. The name is left out when it is not known; the
// code never is.
func recoveryFileURL(name, code string) templ.SafeURL {
	var b strings.Builder
	b.WriteString("Schmetterpause – Wiederherstellungscode\n\n")
	if name != "" {
		b.WriteString("Spieler: " + name + "\n")
	}
	b.WriteString("Code:    " + code + "\n\n")
	b.WriteString("Damit kommst du wieder an deinen Spieler, wenn du deine PIN vergessen hast:\n")
	b.WriteString("auf der Startseite deinen Namen wählen und den Code statt der PIN eintippen.\n")
	b.WriteString("Ein neuer Code aus deinem Profil macht diesen ungültig.\n")

	// PathEscape rather than QueryEscape: a data: URL is not a query, and a
	// space encoded as "+" would arrive in the file as a plus sign.
	return templ.SafeURL("data:text/plain;charset=utf-8," + url.PathEscape(b.String()))
}
