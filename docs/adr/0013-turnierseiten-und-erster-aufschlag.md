# ADR-0013: Im Turnier ist die Tischseite Teil der Paarung

- **Status:** accepted
- **Datum:** 2026-09-03
- **Betrifft:** Turniere, Datenmodell, Regeln
- **Bezug:** legt fest, was `Home`/`Away` im Turnier zusätzlich bedeuten;
  schreibt `0011-finale-und-rueckspiele` insofern fort, als die Auslosung dort
  schon die Orientierung durchwechselt. Ersetzt kein ADR, und ist eine
  benannte Ausnahme zur Hausregel „Der erste Aufschlag wird ausgespielt".

## Kontext

Am Turnierabend passiert zwischen zwei Paarungen jedes Mal dasselbe: zwei
Leute stehen an der Platte und klären, wer wohin geht und wer anfängt. Beim
zweiten Mal ist es Routine, beim zwanzigsten kostet es den Abend.

Die Hausregel dafür ist bewusst rituell: **der erste Aufschlag wird
ausgespielt.** Für ein Spiel zwischen zwei Leuten in der Pause ist das genau
richtig. Bei achtundzwanzig Paarungen an einem Nachmittag ist es
achtundzwanzig Mal Ball einwerfen, bevor gespielt wird.

Was im Datenmodell schon existiert, ist eine Orientierung. `Home` und `Away`
stehen auf jeder Paarung, und die Auslosung wechselt sie absichtlich durch,
damit nicht immer derselbe zuerst genannt wird. Der Kommentar im Domain-Paket
sagte dazu bisher ausdrücklich:

> Home and Away are an orientation, not a venue.

Genau dieser Satz ist die Entscheidung, die hier fällt — und er wird
zurückgenommen.

## Entscheidung

**Im Turnier ist `Home` die eine Tischseite und `Away` die andere. Der
erstgenannte Spieler beginnt auf Seite A und hat den ersten Aufschlag.**

Dazu:

1. **Die Seiten tragen Namen, pro Turnier, Vorgabe `A` und `B`.** Die Platte
   steht nicht immer im selben Raum, also kann kein fester Name stimmen. Wer
   „Fenster" und „Tür" einträgt, bekommt einen Spielplan, den man ohne Schild
   an der Wand lesen kann; wer nichts einträgt, bekommt `A` und `B`, was
   immerhin sagt, dass die beiden Enden auseinanderzuhalten sind.
2. **Zugewiesen, nicht gewürfelt.** Die Zuordnung ist die Auslosung: Seite A
   ist, wer in der Paarung zuerst steht. Damit ist sie über Reloads, Geräte
   und Ausdrucke hinweg dieselbe — was ein `rand()` pro Seitenaufruf nicht
   wäre. Wer „egal, meinetwegen zufällig" sagt, meint genau das: einmal
   entschieden und dann fest.
3. **Kein Speichern der Zuordnung.** Nur die beiden Namen sind neu in der
   Datenbank. Wer wo beginnt, ist eine Funktion der gespeicherten
   Spielerreihenfolge — dieselbe Eigenschaft, die `0011` für das Finale und
   den Spielplan durchgesetzt hat: berechenbar statt kopiert, damit es nicht
   auseinanderlaufen kann.
4. **Die Regelseite nennt die Ausnahme.** Ohne sie widersprechen sich Aushang
   und Spielplan, und der Aushang hängt an der Wand, wo niemand einen
   Kommentar im Code liest.

## Warum nicht anders

**Nur die Seite vorgeben, den Aufschlag weiter ausspielen.** War der erste
Entwurf und ist der halbe Nutzen: das Einwerfen ist der Teil, der die Zeit
kostet, nicht das Seitenfinden. Wer es trotzdem ausspielen will, spielt es
aus — auf einem Zettel steht keine Polizei.

**Zufällig pro Paarung, gespeichert.** Eine Spalte mehr und eine Migration
mehr, für ein Ergebnis, das von der Auslosung nicht zu unterscheiden ist. Die
Auslosung ist schon zufällig: `handleCreateTournament` mischt das Feld, bevor
es gespeichert wird.

**Zufällig pro Paarung, nicht gespeichert.** Der Fehler, den die Schlägerfarbe
schon einmal verhindert hat: eine Seite, die bei jedem Reload eine andere ist,
liest sich als Fehler und nicht als Zufall.

**Seitennamen global konfigurieren.** Wäre eine Umgebungsvariable und
widerspräche dem Fall, der die Frage aufgeworfen hat: die Platte steht nicht
immer im selben Raum. Pro Turnier ist die kleinste Einheit, in der die Antwort
stabil ist.

**Beide Namen erzwingen.** Ein Turnier ohne Seitennamen anzulegen muss möglich
bleiben, sonst kostet ein Feature, das Zeit sparen soll, beim Anlegen Zeit.
Leer heißt Vorgabe.

## Konsequenzen

- Migration `20260903160000_tournament_sides.sql`: zwei Textspalten mit
  Vorgabe, vorwärtsgerichtet und additiv. Jedes bestehende Turnier bekommt
  `A`/`B`, also die Aussage, die es vorher implizit hatte: die Enden sind
  unterschieden, sie haben nur keinen Namen.
- Der Kommentar an `domain.Match` wird korrigiert. Er sagte, `Home` sei kein
  Ort; im Turnier ist er jetzt einer. Außerhalb eines Turniers bleibt er
  reine Formularorientierung — ein Match in der Pause hat keine Auslosung,
  die eine Seite zuweisen könnte.
- Der Spielplan nennt die Seite hinter jedem Namen und erklärt einmal
  darüber, dass der Erstgenannte aufschlägt. Einmal statt achtundzwanzigmal.
- Die Regelseite bekommt den Zusatz an der Aufschlagregel, nicht als siebte
  Regel: es ist eine Ausnahme zu einer bestehenden, und eine eigene Zeile
  würde sie als eigenständige Regel lesen lassen.
