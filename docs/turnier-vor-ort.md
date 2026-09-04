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

Das fragt die vier Werte ab und schreibt `.env`. Die Adresse wird aus einer
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
SP_BOOTSTRAP_ADMIN=Anna
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
sind davon nicht betroffen, die Zuordnung Browser → Spieler schon. Auch PIN und
Wiederherstellungscode überleben einen Schlüsselwechsel — sie sind gesalzen
gehasht und hängen nicht am Schlüssel (`docs/adr/0007`).

**`SP_BOOTSTRAP_ADMIN`** darf leer bleiben. Am Tisch braucht niemand einen
Admin — aber wer während des Abends ein Kiosk-Gerät zurücknehmen will, braucht
einen, und das ist nichts, was man merkt, während jemand danebensteht.

Die Variable nennt einen **Anzeigenamen** und wirkt **beim Start**. Daraus folgt
eine Reihenfolge, die sich nicht umdrehen lässt: die Person tritt erst bei, dann
`task office:up` noch einmal. Vorher gibt es niemanden, dem das Flag gehören
könnte. `office:setup` schreibt das auch noch einmal auf den Bildschirm.

## Starten

```sh
task office:up
```

Die Ausgabe nennt vier Adressen:

- die Startseite mit Rangliste und Ergebniseingabe,
- `/qr` — der Aushang zum Ausdrucken und Ankleben,
- `/rules` — die Hausregeln, derselbe Aushang in Worten,
- `/kiosk?token=…` — einmal am Laptop öffnen, danach merkt sich der Browser das
  für zwölf Stunden. Das Token steht in keiner Antwort mehr, es wird nur gegen
  ein signiertes Cookie getauscht.

Der Aushang ist zum Drucken gemacht: schwarz auf weiß, der Code groß genug, um
aus Armlänge gescannt zu werden. Menü → Drucken reicht, es gibt bewusst keinen
Knopf dafür.

`/rules` ist genauso gebaut und gehört daneben: Aufschlag diagonal, der erste
Aufschlag wird ausgespielt, Aufschlagwechsel nach zwei Punkten, zwei Punkte
Abstand, Netzroller wird wiederholt. Die Zahlen darin stammen aus denselben Regeln wie das
Eingabeformular — der Ausdruck kann dem Formular also nicht widersprechen,
solange beide aus derselben Version kommen. Er hängt nicht am QR-Code und muss
nach einem Adresswechsel auch nicht neu gedruckt werden.

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

**Sag dem Raum, was der Code ist.** Wer beitritt, bekommt einen
Wiederherstellungscode **einmal** angezeigt — Screenshot machen oder in den
Passwortmanager. Ohne diesen Satz verpufft die Anzeige an genau den Leuten,
für die sie da ist. Wer mag, setzt gleich darunter eine PIN; die merkt man
sich, den Code nicht. Beides zusammen ist der Weg zurück, wenn ein Handy
seinen Spieler vergisst — und das passiert.

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

**Oder ohne Token in der Adresse:** `/kiosk` einfach aufrufen. Ohne Freigabe
zeigt die Seite ein Feld für den Zugangscode; eingetippt, freigeschaltet,
fertig. Das ist der Weg, der nichts hinterlässt — ein Token in der Adresse
steht danach in der History und in der Autovervollständigung des Laptops, und
in jedem Chat, in den jemand den Link kopiert. Das Geheimnis ist dasselbe, nur
der Weg ist sauberer.

**Raten kostet Zeit.** Drei Fehlversuche pro Adresse sind frei, danach wächst
die Wartezeit bis auf fünf Minuten und wird nach einer Stunde ohne Fehler
vergessen. Beide Wege — Formular und `?token=` — teilen dieselbe Bremse, sonst
wäre sie keine.
Ab da trägt ein Cookie die Freischaltung, **zwölf Stunden lang** — das Token
selbst steht in keiner Antwort mehr. Neu laden, Tab schließen, Seite wechseln:
alles unproblematisch. Ein anderes Gerät bekommt auf `/kiosk` eine **403**,
solange es den Link mit Token nicht selbst geöffnet hat.

**Danach fragt der Kiosk: wer trägt ein?** Eine Auswahl aus der Spielerliste,
und erst danach zeigt die Seite überhaupt Felder zum Tippen. Der Grund steht
im Ergebnis: ein Kiosk-Ergebnis zählt sofort, ohne dass der Gegner es
bestätigt, und deshalb steht dabei, wer es eingetippt hat. Vorher stand dort
der Heimspieler — was in den Daten aussah, als hätte der sein eigenes Spiel
gemeldet.

Zwei Dinge folgen daraus, und beide sind am Abend spürbar:

- **Das eigene Spiel geht hier nicht.** Wer eintippt, kann kein Match werten,
  in dem er selbst steht — der Kiosk lehnt es ab und verweist auf die
  Startseite, wo der Gegner bestätigt. Das gilt auch im Turnier.
- **Der Laptop wird weitergereicht.** Oben auf der Kiosk-Seite steht, wer
  gerade einträgt, mit einem *Übernehmen* daneben. Wer den Stift übernimmt,
  wählt sich dort aus; die Freischaltung des Geräts bleibt davon unberührt.

Wer gerade an welchem Gerät tippt, steht auch unter `/admin` in der
Geräteliste.

**Jedes Gerät bekommt eine eigene Freigabe.** Zwei Laptops, die das Token
öffnen, halten zwei verschiedene Cookies — und das ist der Unterschied, der
zählt, wenn jemand das Token über die Schulter mitgelesen hat. Unter `/admin`
steht, welche Geräte gerade freigeschaltet sind, wann sie zuletzt gesehen
wurden und wann sie ablaufen; daneben ein Knopf pro Zeile.

- **Ein Gerät zurücknehmen** trifft nur dieses eine. Der Laptop am Tisch läuft
  weiter.
- **Alle zurücknehmen** nimmt den Tisch mit. Das ist der Sinn: es ist für den
  Moment, in dem das Token durchgesickert ist, und dann muss alles weg. Der
  Laptop kommt zurück, indem er das Token erneut eingibt.

Dafür braucht es einen Admin — siehe `SP_BOOTSTRAP_ADMIN` oben.

**Spieler anlegen.** Name eintragen, fertig. Der Spieler bekommt keine Sitzung
— er existiert in der Rangliste und kann eingetragen werden.

**Zugang wiederherstellen.** Weiter unten auf derselben Seite: Spieler wählen,
*Neuen Code ausgeben*. Der Code steht dann **einmal** auf dem Laptop, zum
Vorlesen oder Abschreiben. Damit kommt jemand, der am Kiosk angelegt wurde,
auf sein eigenes Handy — und ebenso jemand, dessen Handy ihn vergessen hat.
Das ist die Stelle, an der der letzte Messlauf gescheitert ist.

Der Kiosk kann **keine PIN** für jemanden setzen. Eine PIN, die jemand anderes
kennt, ist keine. Die vergibt jeder selbst, sobald er wieder angemeldet ist.

Wer sich trotzdem unter einem zweiten Namen einträgt, steht zweimal in der
Rangliste; die beiden zusammenzuführen geht weiterhin nicht.

**Ergebnis eintragen.** Zwei Spieler wählen, Modus einstellen, Sätze eintragen.
Die Zeilen richten sich nach dem Modus: Best of 3 zeigt drei, Best of 7 sieben,
„Ein Satz" eine.

**Diese Ergebnisse zählen sofort.** Keine Bestätigung, keine Warteschlange —
wer sie eintippt, steht neben der Platte und hat das Match gesehen. Die
Rangliste unter dem Formular aktualisiert sich mit.

**Vertippt?** Ein Ergebnis aus dem Kiosk ist sofort gewertet und damit
bestätigt — und ein bestätigtes Match taucht in keiner Liste mehr auf, die man
bestreiten könnte. Der Widerspruchsweg gilt nur für Ergebnisse, die noch auf
eine Bestätigung warten.

Es gibt genau einen Weg zurück: **Zurücknehmen**, direkt in der Meldung, die
das Eintragen erzeugt hat. Zehn Minuten lang, und nur solange für beide
Spieler kein neueres Match gewertet wurde — die Wertung wird zurückgeschrieben,
und das stimmt nur, wenn seitdem nichts passiert ist. Also lieber einmal mehr
hinsehen, bevor du auf *Eintragen und werten* drückst.

### Ein schnelles Turnier

Bis hierher beschreibt dieses Dokument einen Abend, an dem einfach gespielt
wird. Wenn es stattdessen „jeder gegen jeden" sein soll, legt **Turniere →
Schnelles Turnier** einen Spielplan an: Namen ankreuzen, Turnier anlegen. Das
Feld wird gemischt, und daraus entsteht ein Round Robin nach dem
Kreisverfahren — jeder trifft jeden genau einmal, bei ungerader Teilnehmerzahl
setzt in jeder Runde genau einer aus, und über das Turnier gesetzt jeder
einmal.

**Die Form wird einmal gefragt, beim Anlegen.** „Jeder gegen jeden" oder
„Mit Rückspiel", und ob ein **Finale zwischen den besten zwei** folgt. Unter
den Namen steht mit, wie viele Spiele daraus werden und wie lange das dauert —
die Zahl rechnet sich mit, während du ankreuzt, weil sie von drei Antworten
gleichzeitig abhängt. Acht Leute mit Rückspiel sind 56 Spiele; die Grenze von
zwölf Spielern war für den einfachen Fall gedacht.

Das Finale bekommt seinen Platz sofort und seine Namen erst, wenn alle
Gruppenspiele gewertet sind. Stehen die besten zwei dann gleichauf, gibt es
keins — die Seite sagt das, und die geteilten Plätze bleiben stehen. Ein
Endspiel zwischen zwei zufällig gewählten von drei Gleichplatzierten wäre eine
Auslosung mit Publikum. Warum das so entschieden ist, steht in
`docs/adr/0011`.

**Der Modus wird einmal gefragt, beim Anlegen** — Ein Satz, Best of 3, 5 oder
7, und ob bis 11 oder bis 21. Er gilt für alle Spiele des Turniers, und die
Eingabe zeigt danach genau so viele Satzfelder, wie der Modus zulässt. Einmal
statt achtundzwanzigmal: ein Turnier ist eine Verabredung darüber, wie der
Abend gespielt wird, keine Frage pro Paarung. Wer sich vertut, legt das
Turnier neu an — der Modus lässt sich nachträglich nicht ändern, weil sonst
die schon eingetragenen Ergebnisse unter einem anderen stünden als dem, unter
dem sie gespielt wurden.

Die Zahl, an der der Nachmittag hängt, steht unter dem Formular: vier Leute
sind sechs Spiele, acht Leute schon 28. Bei einer Viertelstunde pro Spiel sind
das sieben Stunden. Höchstens zwölf Spieler sind erlaubt, und diese Grenze
existiert genau deshalb.

**Ergebnisse trägt das Gerät an der Platte ein.** Die Turnierseite gibt es
zweimal:

| Adresse | Wer | Was |
| --- | --- | --- |
| `/tournaments/<id>` | alle, auf dem Handy | Spielplan und Tabelle, zum Mitlesen |
| `/kiosk/tournaments/<id>` | das freigeschaltete Gerät | dasselbe, plus ein Eingabefeld pro Paarung |

**Eintragen** klappt sich pro Paarung auf: „Ergebnis eintragen" anklicken, und
darunter stehen dieselben Kästchen wie im normalen Formular — mit Satznummern,
den beiden Namen über den Spalten, einem Schieberegler unter jedem Feld und
dem Regeltext. Eingeklappt, weil ein Spielplan viel öfter gelesen als
beschrieben wird und acht Leute achtundzwanzig davon sind.

Der Grund ist kein Sicherheitsdetail, sondern eins über Cookies: die
Kiosk-Freigabe gilt nur unter `/kiosk`, also kann eine Seite außerhalb davon
gar nicht wissen, dass sie am Tisch steht. Statt die Freigabe zu verbreitern,
liegt die Eingabe dort, wo das Cookie ohnehin hinkommt.

Die Adresse musst du dir nicht merken. Zwei Wege führen hin: **`/kiosk` listet
oben unter „Offene Turniere" jedes offene Turnier**, und auf der Turnierseite
selbst steht **„Ergebnisse eintragen"** als Link. Beendete Turniere stehen
nicht in der Liste — sie nehmen nichts mehr an. Der Weg dahin ist einmal
`/kiosk?token=…` und danach nur noch Klicken.

Der Link funktioniert auf jedem Gerät, aber Eingabefelder zeigt er nur dem
freigeschalteten: die Kiosk-Kopie rendert für alle, nur eben ohne Boxen. Wer
sie nicht sieht, liest dort, woran es liegt.

**Seit ADR-0010 gibt es einen zweiten Weg:** wer angemeldet ist, sieht im
Spielplan an den eigenen Paarungen ein Eingabefeld und trägt sein Ergebnis
selbst ein. Der Gegner bestätigt es, dann zählt es — wie bei jedem
Feierabendspiel. Für andere einzutragen geht weiterhin nur am Gerät an der
Platte; dort steht jemand daneben. Damit läuft ein Turnier auch ohne Laptop,
nur eben mit Bestätigungen, die sich auf die Spieler verteilen.

**Turnierergebnisse vom Kiosk zählen sofort**, wie alle Kiosk-Eingaben — bei 28
Spielen wäre eine Bestätigung pro Match der halbe Abend. Sie bewegen die normale TTR,
und zwar Match für Match; warum nicht veranstaltungsweise verrechnet wird und
wann das fällig würde, steht in `docs/adr/0009`.

**Die Tabelle** sortiert nach Siegen, dann nach dem Ergebnis *unter den
Punktgleichen* — dem Direktvergleich, wenn es zwei sind, der kleinen Tabelle
der Gruppe, wenn es mehr sind —, dann nach Satz- und Punktdifferenz. Wer danach
immer noch gleichauf liegt, teilt sich den Platz; ein `·` neben der Zahl sagt
das. Drei Leute können sich im Kreis schlagen, und dann ist ein geteilter Platz
die einzige ehrliche Antwort.

**Beenden** geht jederzeit, nicht erst wenn alle Spiele drin sind. Das nimmt das Turnier von der
Liste der laufenden Dinge und sonst nichts — an der Wertung hängt es nicht,
die ist längst passiert. Gerade das Turnier, das niemand zu Ende spielt, muss
sich wegräumen lassen — es steht danach unter **„vergangene Turniere"**,
eingeklappt unter der Liste der laufenden.

**Ändern und löschen** gehen, solange **kein einziges Ergebnis** drin steht.
Dann sind Feld und Modus noch eine Entscheidung und keine Aufzeichnung: unter
„Turnier ändern" auf der Turnierseite lässt sich beides korrigieren, und
löschen geht dort und in der Liste. Sobald ein Ergebnis eingetragen ist, ist
Schluss damit — der Spielplan ergibt sich aus der gespeicherten Reihenfolge,
ein nachträglich verschobenes Feld würde eingetragene Ergebnisse in Runden
legen, in denen sie nicht gespielt wurden. Ein gespieltes Turnier wird
**beendet, nicht gelöscht**: `matches.tournament_id` ist `on delete set null`,
das Löschen würde die Ergebnisse als Feierabendspiele zurücklassen — weiter
gewertet, und zurück in der Messung, aus der sie absichtlich herausgenommen
sind.

**„Nochmal ne Runde"** steht daneben, sobald alle Spiele drin sind. Der Link
öffnet das Anlege-Formular mit demselben Feld und demselben Modus; der Name
bleibt leer, damit die beiden Turniere in der Liste auseinanderzuhalten sind.
Es entsteht ein zweites Turnier, keine zusätzliche Runde im ersten — die
Tabelle des ersten bleibt damit lesbar, so wie sie am Ende war. Wer jemanden
abwählen will, der schon weg ist, nimmt vor dem Anlegen das Häkchen raus.

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

`task test:integration` ist dagegen unbedenklich, seit die Integrationstests
eine eigene Datenbank haben (`schmetterpause_test`) und `TruncateAll` sich
weigert, irgendetwas zu leeren, dessen Name nicht auf `_test` endet — Issue
#163. Vorher zeigte der Test-DSN auf genau die Datenbank, auf der hier gespielt
wird. Wer den Task auf einer Maschine laufen lässt, auf der ein Abend läuft,
verliert nichts und startet den Datenbank-Container auch nicht neu.

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

`task office:dod` schließt Turnierspiele deshalb ausdrücklich aus — über
`tournament_id`, nicht mehr über `entered_via`. Das war früher ein
Nebeneffekt: Eingabe ging nur am Kiosk, also trug jede Turnierzeile
`entered_via = 'kiosk'`. Seit ADR-0010 kann sie `'player'` tragen, und der
Ausschluss sagt jetzt, was er meint: ein Spielplan ist eine Erinnerung, egal
wer das Handy gehalten hat. Die Zahlen stehen weiter als eigene Spalte daneben,
statt zu verschwinden.

Seit Issue #71 sieht die Datenbank den Unterschied selbst: jedes Ergebnis trägt
in `entered_via`, ob es ein Spieler selbst eingetragen hat oder der Kiosk.
`task office:dod` zählt nur die eigenen Einträge und weist die Kiosk-Zeilen
daneben aus. Du musst dafür nichts mehr von Hand ausklammern.

`SINCE` bleibt trotzdem nützlich, für den Fall, den keine Spalte sieht: ein
Turnier, das komplett von Handys eingetragen wurde, weil ein Spielplan die
Leute dazu angehalten hat. Das ist immer noch kein freiwilliges Eintragen.

```sh
task office:dod SINCE=2026-08-31   # der erste Tag, der zählen soll
```

`SINCE` ist auch die Antwort auf Daten, die schon dastehen und nicht dazu
gehören — ein Probelauf, ein Test zu zweit, ein Abend von früher. Der andere
Weg ist ein sauberer Start: `task office:backup`, dann `task down` und wieder
hoch. Aber nur, wenn die Sicherung wirklich geschrieben wurde.

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

**„Mein Handy kennt mich nicht mehr."** Das hat den Messlauf vom 26.08.
gekippt: Wer sein Cookie verlor, wurde beim Beitritt unter demselben Namen
abgewiesen und kam nie wieder an seinen Spieler. Jetzt gibt es drei Wege
zurück, in dieser Reihenfolge auszuprobieren:

1. **Anmelden** auf der Startseite — „Schon dabei, aber dieses Gerät kennt dich
   nicht?". Namen wählen, PIN oder Wiederherstellungscode eingeben.
2. **Ein anderer Browser** auf demselben Gerät hat die Sitzung oft noch, denn
   das Cookie hängt am Browser.
3. **Der Kiosk** stellt einen neuen Code aus, wenn die Person danebensteht.
   Der alte gilt dann nicht mehr.

Wer nach vier Fehlversuchen eine Wartezeit bekommt: das ist die Bremse gegen
Durchprobieren, sie läuft von selbst ab und sperrt niemanden dauerhaft aus.

**Das Token hat jemand mitgelesen.** Unter `/admin` stehen die freigeschalteten
Geräte; einzeln zurücknehmen oder alle. Kein Neustart nötig, und das Token
selbst muss nicht geändert werden.
