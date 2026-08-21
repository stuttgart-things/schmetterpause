# ADR-0004: WebAuthn/Passkey als zweiter Faktor — keine serverseitige Biometrie

- **Status:** accepted
- **Datum:** 2026-08-21
- **Betrifft:** Authentifizierung, Datenschutz

## Kontext

Auf dem Whiteboard steht "Biometrische Anmeldung (2FA)". Der naheliegende
Gedanke — Gesichtserkennung über die Handykamera mit Abgleich auf dem Server —
und der tatsächlich sinnvolle Weg unterscheiden sich hier grundlegend, deshalb
wird die Entscheidung explizit festgehalten.

## Entscheidung

Biometrie wird ausschließlich über WebAuthn/Passkeys genutzt. Der Server
speichert einen Public Key und einen Signaturzähler. Face ID, Touch ID oder
Windows Hello entsperren lediglich lokal den privaten Schlüssel im Secure Element
des Geräts.

Eine eigene Gesichts- oder Fingerabdruckerkennung wird nicht gebaut — weder mit
Server-Matching noch mit clientseitigen Embeddings, die den Server erreichen.

Der Passkey ist ein zweiter Faktor bzw. ein Convenience-Login für Wiederkehrer,
kein Ersatz für die Erstregistrierung.

## Begründung

- Eine selbst betriebene Gesichtserkennung verarbeitet biometrische Daten im
  Sinne von Art. 9 DSGVO — eine besondere Kategorie personenbezogener Daten. Das
  zieht ausdrückliche Einwilligung, eine Datenschutz-Folgenabschätzung und in
  einem deutschen Unternehmen mit hoher Wahrscheinlichkeit eine Beteiligung des
  Betriebsrats nach sich. Für eine Büro-Tischtennis-App steht das in keinem
  Verhältnis.
- Bei WebAuthn verlässt kein biometrisches Merkmal das Gerät. Der Server sieht
  nur eine Signatur. Damit entfällt der gesamte Art.-9-Komplex.
- Es existiert eine ausgereifte Bibliothek (`github.com/go-webauthn/webauthn`),
  das Schema ist klein.

## Konsequenzen

- **Positiv:** Kein biometrisches Datum wird jemals gespeichert oder übertragen.
- **Einschränkung:** Die RP ID ist an die Domain gebunden. Ein auf der
  Produktivdomain registrierter Passkey funktioniert nicht unter `localhost` oder
  auf einer abweichenden ACA-Domain. Für die lokale Entwicklung ist eine
  getrennte Registrierung oder ein Dev-Bypass nötig.
- **Einschränkung:** HTTPS ist Pflicht (Ausnahme `localhost`). In Compose
  bedeutet das einen Reverse Proxy mit lokalem Zertifikat.
- **Priorisierung:** Passkeys lösen das schnelle Wiederkommen, nicht die
  Erstanmeldung. Für den Anwendungsfall "an der Platte stehen und schnell ein
  Ergebnis eintragen" ist ein QR-Code plus persistentes Cookie schneller, weil er
  gar keine Interaktion kostet. Passkeys sind deshalb nachrangig gegenüber der
  QR-Eingabe.

## Hinweis zum Bedrohungsmodell

Das realistische Missbrauchsszenario dieser App ist "Kollege trägt aus Spaß ein
falsches Ergebnis ein". Dagegen hilft die Bestätigung des Ergebnisses durch den
Gegner deutlich mehr als starke Authentifizierung. Auth-Härtung sollte nicht als
Lösung für dieses Problem verstanden werden.
