# Turnier vor Ort

Ein Laptop steht an der Platte, alle anderen haben ihr Handy dabei. Der Laptop
betreibt die App und ist gleichzeitig das Gerät, an dem Ergebnisse eingetragen
werden, wenn jemand sein Handy nicht dabeihat oder keine Lust hat, erst etwas
einzurichten.

Es ist dieselbe Anwendung und dasselbe Image wie überall sonst. Der Unterschied
sind drei Umgebungsvariablen und eine zweite Compose-Datei, die sie einfordert.

## Einmalig vorbereiten

```sh
task office:setup
```

Das fragt die drei Werte ab und schreibt `.env`. Die Adresse wird aus einer
Liste **ausgewählt statt getippt** — sie landet im QR-Code auf dem Aushang, und
ein Zahlendreher darin fällt erst auf, wenn das erste Handy nirgends ankommt.

Ist [gum](https://github.com/charmbracelet/gum) installiert, läuft die Abfrage
darüber; ist es das nicht, fragt die Task mit Bordmitteln. Der Abend hängt an
keinem Werkzeug, das jemand mitbringen muss.

Wer es lieber selbst schreibt: `task office:ip` listet die Adressen, und `.env`
sieht so aus.

```sh
SP_PUBLIC_BASE_URL=http://192.168.1.23:8080
SP_KIOSK_TOKEN=turnier2026
SP_SESSION_KEY=<Ausgabe von: openssl rand -hex 32>
```

Gesucht ist die Adresse aus dem Büronetz — typischerweise `192.168.x.x` oder
`10.x.x.x`, nicht `127.0.0.1` und nicht die Docker-Bridges (`docker0`, `br-…`).
`office:setup` sortiert die Bridges nach unten, entscheiden musst du trotzdem.

Die Datei steht in `.gitignore`, wird mit `chmod 600` angelegt und darf dort
auch bleiben.

**`SP_PUBLIC_BASE_URL`** ist die Adresse, die auf dem Aushang im QR-Code landet.
Sie muss gesetzt sein: den Aushang öffnet jemand am Laptop, also sagt die
Anfrage `localhost` — und der gedruckte Code schickte jedes Handy in seinen
eigenen Browser statt zum Laptop.

**`SP_KIOSK_TOKEN`** schaltet `/kiosk` frei. Ohne die Variable ist die Adresse
ein 404, und dann muss jeder Spieler erst selbst etwas am Gerät machen, bevor
das erste Ergebnis eingetragen werden kann.

**`SP_SESSION_KEY`** muss mindestens 32 Zeichen haben und **über den ganzen
Abend derselbe bleiben**. Ändert er sich, ist jeder Browser wieder ein Fremder
und der Kiosk muss neu freigeschaltet werden. Die Ergebnisse in der Datenbank
sind davon nicht betroffen, die Zuordnung Browser → Spieler schon.

## Starten

```sh
task office:up
```

Die Ausgabe nennt drei Adressen:

- die Startseite mit Rangliste und Ergebniseingabe,
- `/qr` — der Aushang zum Ausdrucken und Ankleben,
- `/kiosk?token=…` — einmal am Laptop öffnen, danach merkt sich der Browser das
  für zwölf Stunden. Das Token steht in keiner Antwort mehr, es wird nur gegen
  ein signiertes Cookie getauscht.

Der Aushang ist zum Drucken gemacht: schwarz auf weiß, der Code groß genug, um
aus Armlänge gescannt zu werden. Menü → Drucken reicht, es gibt bewusst keinen
Knopf dafür.

Wenn der Laptop an einem anderen Tag eine andere Adresse bekommt, überschreibt
ein Aufruf die Datei für dieses eine Mal:

```sh
SP_PUBLIC_BASE_URL=http://192.168.1.44:8080 task office:up
```

Dann muss allerdings der Aushang neu gedruckt werden — im alten Code steht die
alte Adresse.

## Während des Turniers

Es gibt zwei Wege ein Ergebnis einzutragen, und sie unterscheiden sich in genau
einem Punkt: wer bestätigt.

### Vom Handy

Aushang scannen, beim ersten Mal den eigenen Namen eintragen, danach nur noch
Ergebnisse. **Der Gegner bestätigt** — erst dann zählt das Match für die
Wertung. Bis dahin steht es unter „Eingetragen", der Gegner sieht es in seiner
Liste, und in der Rangliste ändert sich nichts.

Das ist Absicht: ohne Bestätigung könnte jeder beliebige Ergebnisse eintragen,
und die Tabelle wäre wertlos.

### Am Laptop: der Kiosk

Der Kiosk ist die Antwort auf „ich hab mein Handy nicht dabei" und auf „ich
richte jetzt nicht erst was ein". Eine Person tippt für alle.

**Freischalten.** Einmal die Adresse mit dem Token öffnen, die `office:up`
ausgibt:

```
http://192.168.1.23:8080/kiosk?token=turnier2026
```

Die Seite springt auf `/kiosk` um, das Token verschwindet aus der Adresszeile.
Ab da trägt ein signiertes Cookie die Freischaltung, **zwölf Stunden lang** —
das Token selbst steht in keiner Antwort mehr und ist nicht aus dem Browser
auszulesen. Neu laden, Tab schließen, Seite wechseln: alles unproblematisch.
Nur ein anderes Gerät bekommt auf `/kiosk` eine **403**, solange es den Link
mit Token nicht selbst geöffnet hat.

**Spieler anlegen.** Name eintragen, fertig. Der Spieler bekommt keinen Zugang
und keine Sitzung — er existiert in der Rangliste und kann eingetragen werden.
Wer später doch sein Handy nimmt, legt sich dort einen eigenen Eintrag an; die
beiden zusammenzuführen geht im MVP nicht.

**Ergebnis eintragen.** Zwei Spieler wählen, Modus einstellen, Sätze eintragen.
Die Zeilen richten sich nach dem Modus: Best of 3 zeigt drei, Best of 7 sieben.

**Diese Ergebnisse zählen sofort.** Keine Bestätigung, keine Warteschlange —
wer sie eintippt, steht neben der Platte und hat das Match gesehen. Die
Rangliste unter dem Formular aktualisiert sich mit.

**Vertippt? Dann steht es so.** Ein Ergebnis aus dem Kiosk ist sofort gewertet
und damit bestätigt — und ein bestätigtes Match taucht in keiner Liste mehr
auf, die man bestreiten könnte. Der Widerspruchsweg gilt nur für Ergebnisse,
die noch auf eine Bestätigung warten. Im MVP gibt es dafür keinen Weg zurück;
also lieber einmal mehr hinsehen, bevor du auf *Eintragen und werten* drückst.

### „Warum steht bei Spiele 0?"

Weil das Match noch auf die Bestätigung des Gegners wartet. Die Rangliste zählt
ausschließlich **bestätigte** Matches — TTR, Spiele und Bilanz bewegen sich
alle drei erst danach. Ein Ergebnis aus dem Kiosk ist sofort bestätigt und
taucht sofort auf.

## Danach

```sh
task office:backup   # Datenbank in eine .sql-Datei schreiben
task office:stop     # anhalten, alle Ergebnisse bleiben erhalten
```

`task office:up` startet später mit denselben Daten weiter.

> **`task down` nicht verwenden.** Das ist der Entwicklungsbefehl und löscht das
> Datenbank-Volume mitsamt allen Ergebnissen.

## Die Messung — und warum dieser Abend nicht dazugehört

Die Frage, für die es diese Anwendung gibt, steht in `docs/mvp-plan.md`: über
fünf aufeinanderfolgende Arbeitstage mindestens zehn Matches von mindestens
fünf verschiedenen Spielern, **ohne dass jemand daran erinnert wurde**.

```sh
task office:dod
```

Das gibt die Ergebnisse pro Tag aus und dazu das beste Fenster aus fünf
Arbeitstagen mit einem klaren `PASSED` oder `not yet`. Gezählt werden nur
**bestätigte** Matches — was auf eine Bestätigung wartet, ist noch kein
Ergebnis. Wochenendspiele stehen in der Tagesliste, zählen aber in keinem
Fenster: gefragt sind Arbeitstage.

**Ein Turnierabend ist nicht diese Messung.** Acht Spieler im Modus "jeder
gegen jeden" sind 28 Matches, fast das Dreifache der Hürde. Mitgezählt wäre die
Messung bestanden und hätte nichts gezeigt.

Seit Issue #71 sieht die Datenbank den Unterschied selbst: jedes Ergebnis trägt
in `entered_via`, ob es ein Spieler selbst eingetragen hat oder der Kiosk.
`task office:dod` zählt nur die eigenen Einträge und weist die Kiosk-Zeilen
daneben aus. Du musst dafür nichts mehr von Hand ausklammern.

`SINCE` bleibt trotzdem nützlich, für den Fall, den keine Spalte sieht: ein
Turnier, das komplett von Handys eingetragen wurde, weil ein Spielplan die
Leute dazu angehalten hat. Das ist immer noch kein freiwilliges Eintragen.

```sh
task office:dod SINCE=2026-08-26   # der Tag nach dem Turnier
```

Dass niemand erinnert wurde, steht weiterhin in keiner Spalte. Das weißt nur
du.

## Was schiefgehen kann

**Die Handys erreichen den Laptop nicht.** Viele Gäste- und Firmen-WLANs
trennen die Clients voneinander (Client Isolation) — dann sehen die Handys das
Internet, aber nicht den Laptop daneben. Das ist die wahrscheinlichste Ursache
und lässt sich nicht in der App reparieren. Vor dem Turnier mit einem Handy die
Adresse aufrufen, nicht erst, wenn alle dastehen. Falls es scheitert: ein
anderes WLAN, ein Hotspot vom Laptop aus oder ein Kabel.

**Der Laptop schläft ein.** Mit zugeklapptem Deckel ist das Turnier vorbei.
Energieeinstellungen vorher auf „nie" stellen und das Netzteil anschließen.

**Die Adresse ändert sich.** Der Router vergibt per DHCP, und nach einem
Neustart kann eine andere Adresse herauskommen. Dann zeigt der ausgedruckte
Code ins Leere. Entweder eine feste Adresse im Router reservieren oder den
Aushang neu drucken.

**Die Firewall blockt Port 8080.** Unter Linux und macOS meist kein Thema, unter
Windows fragt die Firewall beim ersten Start — die Freigabe für private Netze
muss bestätigt werden.

**Es läuft über HTTP, nicht HTTPS.** Im lokalen Netz für einen Abend in
Ordnung, und die Cookies sind entsprechend ohne `Secure` gesetzt. Ins Internet
gehört diese Konfiguration nicht.
