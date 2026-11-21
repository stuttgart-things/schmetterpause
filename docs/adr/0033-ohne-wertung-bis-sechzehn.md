# ADR-0033: Ein Turnier ohne Wertung darf bis sechzehn Spieler haben

- **Status:** accepted
- **Datum:** 2026-11-20
- **Betrifft:** Turniere, Wertung
- **Bezug:** supersedes `0011-finale-und-rueckspiele` und
  `0009-schnelles-turnier-wertet-pro-match` jeweils nur im Punkt
  „Obergrenze von zwölf Spielern“, und nur für Turniere ohne Wertung
  (ADR-0012). Für gewertete Turniere bleibt alles, wie es ist.

## Kontext

Am 19. November haben vierzehn Leute einen lockeren Abend am Kiosk gespielt.
Das Turnierformular hat sie nicht gelassen: höchstens zwölf.

Die Zwölf hat zwei Begründungen, und sie sind verschieden stark:

- **Tischzeit** (#41, ADR-0011). Zwölf Spieler sind 66 Spiele, bei einer
  Viertelstunde pro Spiel gut sechzehn Stunden. Das ist ein Argument, aber
  eines, das das Formular schon selbst führt: Es rechnet beim Ankreuzen mit,
  wie viele Spiele und Stunden entstehen. Wer mit zwei Platten, einem Satz bis
  11 oder dem Plan spielt, nicht fertig zu werden, kann die Zahl selbst
  bewerten.
- **Wertung** (ADR-0009). Ein Turniermatch wertet einzeln, nicht
  veranstaltungsweise. Ab etwa zwölf Spielern verschiebt sich die TTR über den
  Abend so weit, dass frühe Spiele mit anderen Erwartungswerten gerechnet
  werden als späte. ADR-0009 nennt genau dieses Feld als Auslöser, die
  veranstaltungsweise Wertung zu bauen. Das ist ein Argument gegen größere
  gewertete Felder, und es steht.

Ein Turnier ohne Wertung (ADR-0012) bewegt keine TTR. Die zweite Begründung
fällt dort weg, und übrig bleibt die erste, die das Formular schon anzeigt.

## Entscheidung

**Mit Wertung höchstens zwölf, ohne Wertung höchstens sechzehn.**

- Die beiden Grenzen liegen als `tournament.MaxRatedPlayers` und
  `tournament.MaxUnratedPlayers` im fachlichen Paket, nicht im Handler.
  Anlegen, Ändern, der mitlaufende Zähler und der Knopf fragen alle
  `tournament.MaxPlayers(rated)`.
- Das Häkchen „Zählt nicht für die Rangliste“ zeichnet den Zähler neu wie ein
  angekreuzter Name: „14 von höchstens 16“ statt „14 von höchstens 12“.
- Wer gewertet zwischen 13 und 16 ankreuzt, bekommt nicht nur „zu viele“,
  sondern den Ausweg: „14 Spieler gehen nur ohne Wertung.“ Der Ausweg ist ein
  Häkchen entfernt, also wird er genannt.
- Sechzehn ist wieder eine Tischzeitgrenze: 120 Spiele, an einer Platte
  dreißig Stunden. Größer wird ein Abend nur mit einer anderen Turnierform,
  nicht mit einem größeren Jeder-gegen-jeden.

## Warum nicht anders

**Sechzehn für alle.** Dann tritt der Auslöser aus ADR-0009 ein, ohne dass die
veranstaltungsweise Wertung gebaut wird. Eine Büromeisterschaft mit sechzehn
Leuten würde die TTR mit der Ungenauigkeit verstellen, die ADR-0009 bis zwölf
bewusst in Kauf genommen hat, darüber aber nicht.

**Die Grenze ganz streichen und nur warnen.** Das Formular warnt schon mit
der Stundenzahl. Ein Feld von zwanzig ist trotzdem kein Abend mehr, sondern
ein Fall für das Schweizer System, das ADR-0011 als nächsten Schritt nennt.
Eine harte Grenze macht diese Frage sichtbar, statt sie auf einen Abend zu
verschieben, der nicht fertig wird.

**Schweizer System jetzt.** Das ist die richtige Antwort für große Felder und
eine eigene Entscheidung. Für vierzehn Leute an einem lockeren Abend reicht
ein größeres Jeder-gegen-jeden ohne Wertung.

## Konsequenzen

- Ein Spaßturnier mit bis zu sechzehn Leuten lässt sich anlegen. Ob es fertig
  wird, sagt das Formular vorher in Stunden.
- Ein gewertetes Turnier mit mehr als zwölf Leuten bleibt unmöglich, bis die
  veranstaltungsweise Wertung aus ADR-0009 gebaut ist. Dann ist diese Grenze
  die erste, die neu verhandelt wird.
- Ein Turnier ohne Wertung mit mehr als zwölf Spielern lässt sich nicht in
  ein gewertetes ändern, solange es noch nicht gespielt ist: Die Änderung
  scheitert an derselben Prüfung wie das Anlegen.
