# ADR-0006: Wiederherstellungscode statt Login

- **Status:** accepted
- **Datum:** 2026-08-25
- **Betrifft:** Authentifizierung, Betrieb
- **Bezug:** schreibt `0003-identitaeten-eigene-tabelle` und
  `0004-webauthn-keine-serverseitige-biometrie` fort, ersetzt keins von beiden

## Kontext

Wiedererkennung läuft heute über ein signiertes Cookie (ADR-0003): QR scannen,
Namen eintragen, fertig — **null Interaktion** bei jedem weiteren Besuch. Das ist
die Eigenschaft, die AP7 misst, und sie steht nicht zur Disposition.

Beim Handytest vor dem Turnier ist der Fall aufgetreten, den niemand geplant
hatte: das Cookie verschwand, und danach gab es **keinen Weg zurück**. Beitreten
mit demselben Namen wird abgelehnt, und `handleJoin` ist der einzige Weg zu einer
Sitzung. Der Spieler behält Wertung, Historie und Platz in der Rangliste — und
kann nie wieder als er selbst handeln (Issue #70).

Das ist kein Sicherheitsproblem. Das Bedrohungsmodell aus ADR-0004 gilt
unverändert: das realistische Missbrauchsszenario ist "Kollege trägt aus Spaß
ein falsches Ergebnis ein", und dagegen hilft die Bestätigung durch den Gegner.
Was fehlt, ist kein Nachweis, sondern ein **Weg zurück**.

Es trifft aber die Messung. Die Definition of Done fragt, ob Leute freiwillig
eintragen (#7). Wer nicht mehr an seinen Spieler kommt, trägt nichts mehr ein —
und die Messung liest das als Desinteresse.

## Entscheidung

Ein **Wiederherstellungscode**. Generiert, nicht gewählt. Ersetzbar, nicht
dauerhaft.

- Beim Anlegen **einmalig angezeigt**, mit dem Hinweis, ihn dort zu speichern, wo
  die Passwörter liegen.
- **Für sich selbst** darf jeder einen erzeugen: beim Anlegen, und danach
  jederzeit aus der eigenen angemeldeten Sitzung.
- **Für jemand anderen** nur der Kiosk. Das ist die Wiederherstellung für
  Leute, die nichts mehr haben.
- **Ein neuer Code macht den alten sofort ungültig.**
- **Kein Passwortfeld.** Der Code wird nie selbst gewählt.
- Als **Code**, nicht als Link, nicht als Datei, nicht als Wallet-Pass.
- Serverseitig **gehasht**, nie im Klartext.
- Das Cookie bleibt der Hauptweg. Der Code ist der Weg, den man nur trifft, wenn
  etwas kaputtgegangen ist.

## Begründung

### Generiert, nicht selbst gewählt

Ein Feld, in das man einen Code eintippen *kann*, wird ein Feld, in das jemand
sein **Firmenpasswort** tippt. Eine Büro-Tischtennis-App, die Passwörter
einsammelt, wäre ein Schaden ohne jedes Verhältnis zu dem, was sie sonst tut.

Dazu das kleinere Argument: bei einem generierten Code ist die Entropie bekannt,
bei einem gewählten wird sie geraten — und geraten wird sie zu hoch.

### Ein Code, kein Link

**Chat-Programme rufen Links auf, um Vorschauen zu bauen.** Ein einmal gültiger
Link, den jemand in Teams einfügt, kann vom Vorschau-Roboter verbraucht werden,
bevor ein Mensch ihn öffnet. Eine Zeichenfolge, die man vorliest oder abtippt,
passiert das nicht.

Ein Link landet außerdem im Verlauf, in Lesezeichen und in geteilten Tabs.

### Keine Datei, kein Wallet

Ein Artefakt, das man **nur im Schadensfall braucht**, ist zu diesem Zeitpunkt
meist schon weg: Handy gewechselt, Downloads aufgeräumt, "welche Datei war das
nochmal". Genau dieses stille Verlieren ist ja der Grund, warum #70 existiert —
ihm ein zweites Ding zum Verlieren entgegenzusetzen, löst es nicht. Ein
Passwortmanager wird nicht aufgeräumt, ein Download-Ordner schon.

Apple Wallet braucht ein signiertes `.pkpass`, also einen Developer-Account und
ein Pass-Type-Zertifikat **im Container**; Google Wallet ein Issuer-Konto. Issue
#37 kommt zum selben Schluss und empfiehlt stattdessen "als Bild speichern".

Und ein QR-Bild kann sich das eigene Handy nicht selbst zeigen: auf einem neuen
Gerät bräuchte es eine zweite Kamera oder einen Datei-Upload.

### Ersetzbar statt dauerhaft

Ein dauerhaftes Token kann man nur entwerten, indem man den Besitzer mit
aussperrt. Ein ersetzbarer Code hat einen Rücknahmeweg, der nichts kostet: wer
den Verdacht hat, dass sein Code irgendwo gelandet ist, erzeugt einen neuen.

### Für sich selbst ja, für andere nur am Kiosk

Der Verdacht liegt nahe, dass ein neuer Code eine Admin-Handlung sein müsste.
Er zeigt aber in die falsche Richtung.

**Einen Code für sich selbst zu erzeugen, ist keine Rechteerweiterung.** Wer das
tut, hält bereits eine gültige Sitzung — er *hat* den Zugang. Die Handlung
verschafft ihm nichts Neues, sie nimmt dem alten Code die Gültigkeit. Das ist
"Passwort ändern, während man angemeldet ist", und daraus eine fremde
Entscheidung zu machen, macht es schlechter: wer den Verdacht hat, sein Code sei
irgendwo gelandet, müsste erst zum Laptop laufen. In einer normalen Woche steht
da keiner, also bliebe der geleakte Code bis zum nächsten Turnierabend gültig.

**Mächtig ist die andere Richtung:** einen Code für einen Spieler ausstellen,
der nicht dabei ist. Das kann nur der Kiosk, und dort ist die Bedingung der
Raum — jemand steht daneben, und die Leute kennen sich. Genau deshalb bleibt
dieser Weg an den Kiosk gebunden und wandert nicht in die normale Oberfläche.

### Drei Ausgabestellen, nicht eine

**In der Messwoche läuft kein Kiosk an der Platte.** Ein Weg zurück, den es nur
am Turnierabend gibt, ist von Mittwoch bis Dienstag keiner. Deshalb muss die
eigene angemeldete Sitzung selbst einen ausgeben können.

Wer Code *und* alle angemeldeten Geräte verloren hat, braucht den Kiosk. Das ist
Absicht: an dieser Stelle soll ein Mensch danebenstehen, und das Vertrauen kommt
aus dem Raum.

### Warum nicht OIDC zuerst

`web/web.go` begründet, warum Schriften und HTMX im Binary liegen: die Anwendung
muss **auf einem Netz hochkommen, das das Internet nicht erreicht**. Ein Login
gegen einen Identity Provider gibt genau das auf — ohne Internet meldet sich
niemand an, und der Abend an der Platte (`docs/turnier-vor-ort.md`) ist exakt
dieser Fall.

Dazu: eine Client-Registrierung ist ein Ticket bei Leuten mit anderen
Prioritäten, und eine Weiterleitung am Eingang widerspricht dem
Interaktionsbudget aus AP7.

OIDC bleibt der richtige Endzustand, sobald Identität wirklich stimmen muss —
eine Liga, an der etwas hängt. Bis dahin löst es ein Problem, das wir nicht
haben, und schafft eins, das wir hatten.

### Warum nicht Passkeys zuerst

ADR-0004 sagt es bereits: Passkeys lösen das **Wiederkommen**, nicht die
Erstanmeldung. Jemand muss die Identität einmal herstellen.

Dazu die harte Vorbedingung: WebAuthn verlangt HTTPS, und die RP ID muss eine
**Domain** sein — eine IP-Adresse ist als RP ID nicht zulässig. Auf
`http://10.x.x.x:8080` gibt es kein WebAuthn, nicht schlecht, sondern gar nicht.

Sobald ein Hostname mit Zertifikat steht, sind Passkeys die nächste Schicht auf
diesem ADR — und dann billiger als OIDC, weil sie von niemandem eine Freigabe
brauchen.

## Konsequenzen

- **Positiv:** kein neuer Dienst, keine Abhängigkeit, keine Freigabe von Dritten,
  funktioniert ohne Internet. Additiv zum Schema (Invariante 8), und weil Auth
  hinter einer Schnittstelle liegt (Invariante 4), ändert sich kein Handler.
- **Der Code ist ein Bearer-Credential.** Wer ihn hat, ist der Spieler. Die
  Sprengweite ist die aus ADR-0004: ein falsches Ergebnis, das der Gegner
  bestätigen muss. Diese Grenze darf nicht überschritten werden — der Code darf
  nie die Grundlage für etwas mit echten Folgen werden.
- **Einschränkung:** ein Code, der einmal angezeigt und nicht gespeichert wurde,
  ist weg. Die Anzeige muss deshalb eindeutig sein und darf nicht wie eine
  Bestätigungsmeldung aussehen, über die man hinwegliest.
- **Einschränkung:** Code weg *und* alle Geräte weg heißt Kiosk. Ohne Kiosk ist
  der Spieler nicht wiederherstellbar.
- **Nötig:** Begrenzung der Versuche pro Spieler und pro Adresse. Ein kurzer Code
  lädt zum Durchprobieren ein, und ohne Bremse ist die Länge das einzige, was
  zwischen jemandem und einem fremden Spieler steht.

## Offene Punkte

Bewusst nicht entschieden, weil sie die Entscheidung oben nicht berühren:

1. **Länge und Alphabet.** Verwechselbare Zeichen gehören raus (`0`/`O`, `1`/`l`),
   weil der Code vorgelesen und abgetippt wird.
2. **Läuft der Code ab?** Neigung: nein, er gilt bis zum Ersetzen. Ein
   Wiederherstellungscode, der abläuft, ist einer, der genau dann nicht geht,
   wenn man ihn braucht.
3. **Wie wird nachgeschlagen?** Ein Passwort-Hash mit eigenem Salt pro Zeile
   erzwingt einen Scan über alle Spieler, weil man ohne Kandidaten nicht weiß,
   gegen welchen Salt zu prüfen ist. Bei einigen Dutzend Spielern ist das
   folgenlos, aber es ist eine Entscheidung: Scan mit Salt, oder ein
   deterministischer Keyed Hash (HMAC mit dem Session-Key), der einen Index
   erlaubt und dafür bei einem Schlüsselwechsel alle Codes entwertet.
4. **Ein Code pro Spieler oder mehrere?** Mehrere wären "ein Code pro Gerät" und
   damit näher an Passkeys — aber auch mehr, was verloren gehen kann.
5. **Es gibt keine Administration.** Dieses ADR lehnt sich für "einen Code für
   jemand anderen" an den Kiosk an, und der ist als Fundament dünn: sein Cookie
   ist ein konstanter Wert aus Session-Key und Token, in jedem Browser identisch,
   der die Token-Adresse je geöffnet hat, zwölf Stunden gültig — und
   zurücknehmen lässt er sich nur, indem man das Token ändert und neu startet.
   Für diesen einen Zweck reicht das. Phase 2 bringt weitere Handlungen, die
   jemanden brauchen, dem man das zutraut, und dann ist es keine Frage der
   Codes mehr. Eigenes Thema.
