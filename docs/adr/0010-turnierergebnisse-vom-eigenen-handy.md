# ADR-0010: Turnierergebnisse dürfen vom eigenen Handy kommen

- **Status:** accepted
- **Datum:** 2026-09-01
- **Betrifft:** Turniere, Wertung, Messung
- **Bezug:** schreibt `0009-schnelles-turnier-wertet-pro-match` fort und ändert
  die Auswertung von Issue #7; ersetzt kein ADR

## Kontext

Ein Turnierergebnis kann heute nur das freigeschaltete Gerät an der Platte
eintragen. `docs/turnier-vor-ort.md` nennt dafür einen Grund über Cookies: die
Kiosk-Freigabe gilt nur unter `/kiosk`, also kann eine Seite außerhalb davon
nicht wissen, dass sie am Tisch steht, und statt die Freigabe zu verbreitern
liegt die Eingabe dort, wo das Cookie ohnehin hinkommt.

Das beschreibt den Mechanismus richtig, ist aber die Folge einer früheren
Entscheidung und nicht ihr Grund. Der Grund, der trägt, steht daneben: **bei
28 Spielen wäre eine Bestätigung pro Match der halbe Abend.** Kiosk-Eingaben
zählen sofort, und deshalb liegt die Turniereingabe auf dem Pfad, der sofort
wertet.

Diese Rechnung übersieht eine dritte Möglichkeit. Die Bestätigungen
verschwinden nicht, wenn ein Spieler selbst einträgt — sie verteilen sich auf
die Leute, die gerade gespielt haben, statt sich an einem Laptop zu stauen.
Bei acht Spielern bestätigt jeder sieben Ergebnisse über einen Nachmittag,
nicht 28 hintereinander.

Drei Beobachtungen, die den Anlass ausmachen:

**Ein Turnier setzt heute einen Laptop voraus.** Vier Leute mit Handys um eine
Platte können keins spielen, obwohl der Spielplan auf jedem dieser Handys
lesbar ist. Die einzige Sache, die man auf der Seite nicht tun kann, die einem
sagt, gegen wen man als Nächstes spielt, ist zu sagen, wie es ausging.

**Und die Voraussetzung ist strenger, als sie aussieht.** Ohne
`SP_KIOSK_TOKEN` werden die `/kiosk`-Routen gar nicht erst gemountet — der
Kiosk existiert dann nicht, statt unfreigeschaltet zu existieren. Auf jedem
Deployment ohne diese Variable ließ sich ein Turnier also anlegen und lesen,
aber **nie spielen**. Nicht „unbequem ohne Laptop", sondern funktionslos.
Dieser Punkt kam aus PR #123, der dieselbe Änderung parallel vorgeschlagen
hat.

**Die halb gebaute Variante existiert schon und führt in die Irre.**
`tournamentIDFrom` liest ein `tournament_id` aus dem Formular, und der
Kommentar behauptet, das Feld sei "optional auf jedem Eingabepfad". Es wird
ausschließlich in `internal/server/kiosk.go` aufgerufen; `POST /matches` liest
es nie. Ein Spieler kann also gegen seinen Turniergegner spielen und eintragen
— das Ergebnis zählt für TTR und Rangliste und taucht in der Turniertabelle
nie auf. Von den drei möglichen Verhalten ist das das schlechteste, weil es
wie ein Fehler aussieht statt wie eine Absage.

**Die Messung hängt an einem Nebeneffekt.** Jede Turnierzeile trägt heute
`entered_via = 'kiosk'` und fällt damit aus `task office:dod` heraus. Das ist
gewollt — 28 Matches an einem Abend würden die Hürde aus Issue #7 dreifach
reißen und nichts zeigen —, aber es ist als Ausschluss nirgends formuliert. Er
gilt, weil das Formular an einer bestimmten Adresse liegt.

## Entscheidung

**Ein Spieler darf sein eigenes Turnierergebnis vom eigenen Gerät eintragen;
der Gegner bestätigt es wie bei jedem anderen Match.** Der Kiosk bleibt, was
er ist, und bleibt der schnellere Weg.

Konkret:

- `POST /matches` wertet `tournament_id` aus. Beide Spieler müssen im Feld des
  Turniers stehen — dieselbe Prüfung, die `handleTournamentRecord` schon macht,
  denn ohne sie bucht der Endpunkt beliebige zwei Leute in fremde Turniere.
- Der Modus kommt aus dem Turnier, nicht aus dem Formular. Das gilt am Kiosk
  bereits und gilt hier aus demselben Grund.
- `entered_via` bleibt `'player'`. Die Spalte sagt, wer getippt hat, und das
  bleibt wahr.
- Die Turniertabelle zeigt ein eingetragenes, noch unbestätigtes Match als
  wartend an. Sie kann das bereits — `TournamentRepository.Matches` liefert
  bewusst jeden Status —, aber "0 von 6 gewertet" darf dann nicht mehr die
  einzige Zahl auf der Seite sein.
- **`scripts/definition-of-done.sql` schließt Turniermatches über
  `tournament_id is not null` aus, nicht mehr über `entered_via`.** Das ist
  Teil dieser Entscheidung, kein Folgeticket.

## Konsequenzen

**Der Ausschluss aus der Messung wird ehrlich.** Heute fallen Turnierspiele
heraus, weil das Formular an einer Adresse liegt, die `entered_via = 'kiosk'`
schreibt. Danach fallen sie heraus, weil sie zu einem Turnier gehören — was
der tatsächliche Grund ist. `docs/turnier-vor-ort.md` sagt schon heute, dass
ein Turnierabend nicht diese Messung ist; nach dieser Änderung steht das auch
in der Abfrage.

**Ohne diese Änderung an der Abfrage ist die Entscheidung schädlich.** Acht
Spieler, die ihre Ergebnisse selbst eintragen, produzieren 28 Zeilen mit
`entered_via = 'player'` an einem Tag. Die Definition of Done aus #7 wäre
bestanden und hätte nichts gezeigt: ein Spielplan ist genau die Erinnerung,
die die Messung ausschließen will. Die beiden Teile gehören deshalb in
dieselbe Änderung.

**Die Reihenfolgeabhängigkeit aus ADR-0009 wird größer.** Dort wurde
akzeptiert, dass Turniermatches einzeln und sofort werten und das Ergebnis
damit von der Eingabereihenfolge abhängt — bei einstelligen TTR-Verschiebungen
über einen Nachmittag vertretbar. Mit Bestätigungen fällt die Wertung nicht
mehr mit dem Spiel zusammen, sondern mit der Bestätigung, und die kann eine
Stunde später kommen. Der Effekt bleibt derselbe Größenordnung, aber der
Abstand zwischen "gespielt" und "gerechnet" wächst. Das ist der Preis, und er
ist nicht null.

**Ein Turnier kann jetzt halb sofort und halb wartend sein.** Wer am Kiosk
einträgt, wertet sofort; wer vom Handy einträgt, wartet. Dieselbe Tabelle
zeigt beides. Das ist erklärbar, aber es ist ein Zustand mehr als heute, und
die Seite muss ihn benennen statt ihn zu verschweigen.

**Die Asymmetrie aus #90 bleibt bestehen.** Am Kiosk kann weiterhin jeder sein
eigenes Ergebnis ohne Bestätigung eintragen. Diese Entscheidung macht das
nicht besser und nicht schlimmer; sie stellt nur daneben einen Weg, der
bestätigt wird.

**Drei Punkte, die die Umsetzung mitentschieden hat.** Keiner davon braucht
ein eigenes ADR, aber sie sollen nicht ungeschrieben bleiben:

1. **Ein bestrittenes Match** (`disputed`) steht im Spielplan als *bestritten*,
   nicht als *wartet auf Bestätigung*. Es zählt für nichts und wird das ohne
   eine Korrektur auch nicht — eine Bestätigung anzukündigen, die nicht kommt,
   wäre ein Versprechen.
2. **Schließen bleibt jederzeit möglich**, auch mit unbestätigten Ergebnissen.
   An der Wertung hängt nichts (ADR-0009), und ein Turnier, dessen letztes
   Match nie bestätigt wird, darf nicht ewig offen bleiben. Die betroffenen
   Ergebnisse bleiben ausstehend und können danach noch bestätigt werden.
3. **Nur die eigenen Paarungen.** Wer eingeloggt ist, sieht ein Eingabefeld an
   den Spielen, in denen er selbst steht, und an keinem anderen. Für andere
   einzutragen ist das, wofür das Gerät an der Platte da ist: dort steht
   jemand daneben, und genau das ist bei einem Handy quer durch den Raum nicht
   prüfbar.

Der irreführende Kommentar an `tournamentIDFrom` — "optional auf jedem
Eingabepfad" für ein Feld, das nur ein Pfad las — ist damit nachträglich wahr
geworden und entsprechend korrigiert.

## Verworfene Alternativen

**Beim Kiosk bleiben.** Das Billigste, und es hält die Messung ohne
Änderung sauber. Verworfen, weil es die Voraussetzung Laptop festschreibt und
die halb verdrahtete Stelle stehen lässt, die heute schon dazu führt, dass ein
Turnierspiel als Feierabendspiel in der Rangliste landet, ohne dass jemand das
wollte. Ein Weg, der nur deshalb nicht benutzt wird, weil er unfertig ist, ist
kein Weg, den man verworfen hat.

**Spieler-Eingabe ohne Bestätigung, wie am Kiosk.** Wäre schneller und
bräuchte keine Zustände in der Tabelle. Verworfen, weil es aus #90 — jeder
kann sein eigenes Ergebnis ohne Gegenprüfung werten lassen — von einem
bekannten Loch am Kiosk ein Merkmal der ganzen Anwendung machen würde. Der
Kiosk kann sich das leisten, weil jemand danebensteht.

**Spieler-Eingabe nur, wenn kein Kiosk freigeschaltet ist.** Klingt nach dem
Besten aus beidem. Verworfen, weil dann dieselbe Seite je nach einem Zustand,
den der Betrachter nicht sehen kann, etwas anderes tut. Ein Formular, das
manchmal da ist, ist schlechter als eins, das nie da ist.

**Die Kiosk-Freigabe auf die ganze Anwendung ausweiten**, damit
`/tournaments/<id>` sie sehen kann. Verworfen, weil `Path=/kiosk` die eine
Eigenschaft ist, die den Kiosk eingrenzbar macht: ein Gerät, das freigeschaltet
wurde, kann heute nur unter einer Adresse für andere handeln. ADR-0008 hängt
daran.
