# ADR-0022: Ein Beobachter spielt nicht

- **Status:** accepted
- **Datum:** 2026-09-16
- **Betrifft:** Datenmodell, Authentifizierung, Oberfläche, Schnittstellen
- **Bezug:** schreibt `0008-wer-fuer-andere-handeln-darf` fort und ersetzt es
  nicht — das Admin-Flag bleibt eine Stufe, keine Rolle. Berührt
  `0015-zaehlwerk-traegt-ergebnisse-ein` bei `GET /api/players`. Issue #271.

## Kontext

ADR-0008 hat `players.is_admin` eingeführt, und die Oberfläche unter `/admin`
existiert: Ergebnisse entfernen, Spieler entfernen, Kiosk-Freigaben
zurücknehmen. **Im Büro hat das Flag niemand.** Als am 2026-09-16 ein
Test-Match aus dem Zählwerk wieder heraus musste, lief das deshalb als
SQL-Transaktion von Hand (stuttgart-things#2997) — genau der Weg über `psql`,
den ADR-0008 abschaffen wollte.

Das Flag einem der Spieler zu geben liegt nahe und passt nicht. Wer ein
gewertetes Ergebnis entfernt, soll nicht zugleich eigene Ergebnisse in
derselben Tabelle haben; die Frage „hat da jemand sein eigenes Match
korrigiert" soll sich gar nicht stellen. Gewünscht ist ein **eigenes Konto,
das verwaltet und nie spielt** — Name `timoboll`.

Ein solches Konto ist heute ein gewöhnlicher Spieler. Es stünde mit TTR 1000 in
der Rangliste, wäre als Gegner wählbar, im Turnier aufstellbar und läge über
`GET /api/players` in der Operator-Auswahl des Zählwerks.

## Entscheidung

**Ein zweites Flag am Spieler, `players.is_observer`: wer es trägt, spielt
nicht. Es sagt nichts über Rechte.**

### Was ein Beobachter nicht ist

- **Nicht in der Rangliste** und nicht in den Statistiken.
- **Nicht wählbar als Spieler**: nicht als Gegner beim Eintragen, nicht im Kiosk
  als einer der beiden Spieler, nicht im Turnier.
- **Nicht in `GET /api/players`.** Das Zählwerk bleibt, wie es ist.
- **Abgelehnt als Spieler** in jedem schreibenden Weg — Spieler-Pfad, Kiosk,
  Turnier, `POST /api/results`. Die Oberfläche blendet ihn aus, die Prüfung
  sitzt trotzdem beim Schreiben, weil eine ausgeblendete Option keine Regel ist.

### Was ein Beobachter bleibt

- **Er meldet sich an** wie jeder andere: Name zuerst, PIN (ADR-0007, ADR-0018).
  Die Anmelde-Auswahl zeigt ihn deshalb weiter. „Alle Spieler" und „alle, die
  spielen" sind ab hier zwei verschiedene Listen, und jede Stelle, die heute
  `Players().List` liest, muss sagen, welche sie meint.
- **Er darf Operator am Kiosk sein.** Jemand, der zusieht und zählt, ist genau
  das, was ADR-0014 vom Operator verlangt.
- **Er kann Admin sein.** `timoboll` trägt beide Flags. Die beiden sind
  unabhängig: ein Beobachter ohne Admin-Rechte ist ein gültiger Fall, etwa ein
  Konto für jemanden, der nur zusieht.

### Wer es setzen darf, und wann

- **Ein Admin, unter `/admin`**, auch für das eigene Konto. Jede Änderung
  schreibt eine Logzeile mit beiden `player_id`, wie jede Admin-Handlung.
- **Nur für einen Spieler ohne ein einziges Match** — gleich welcher Status —
  und ohne Turnierteilnahme. Eine Historie macht aus einem Spieler keinen
  Beobachter; sie verschwände sonst aus Rangliste und TTR-Verlauf anderer, die
  gegen ihn gespielt haben. Für diesen Fall ist Zusammenführen gedacht
  (ADR-0017, noch Entwurf), nicht dieses Flag.
- **Zurücknehmen geht immer.** Der Spieler taucht mit seinem unveränderten TTR
  wieder auf; da er nie gespielt hat, ist das 1000.

### Der erste Beobachter

Kein neuer Bootstrap. Die Reihenfolge für `timoboll`:

1. Das Konto tritt über die normale Oberfläche bei und setzt dabei **seine
   eigene** PIN.
2. `SP_BOOTSTRAP_ADMIN=timoboll`, Neustart — das Admin-Flag kommt wie in
   ADR-0008 vorgesehen.
3. `timoboll` setzt unter `/admin` sich selbst als Beobachter.

Zwischen 1 und 3 steht das Konto kurz als Spieler ohne Matches am Ende der
Rangliste. Wird es in diesen Minuten als Gegner gewählt, verweigert Schritt 3
das Flag, und das ist sichtbar statt still.

## Warum nicht anders

**Warum kein Wert in einer Rollen-Spalte.** ADR-0008 sagt „ein Flag, keine
Rollen" und meint Stufen von Rechten. `is_observer` ist keine Stufe — es regelt
Teilnahme, nicht Befugnis, und steht quer zu `is_admin`. Eine Spalte
`role in ('player', 'admin', 'observer')` würde zwei unabhängige Fragen in eine
zwingen und `timoboll` genau den Fall verbieten, für den es gebraucht wird.

**Warum der Beobachter nicht aus `SP_BOOTSTRAP_ADMIN` folgt.** Wer das Büro
verwaltet, darf trotzdem mitspielen wollen. Beide Flags an eine Variable zu
koppeln, spart eine Zeile Oberfläche und nimmt diese Freiheit.

**Warum keine eigene Variable `SP_BOOTSTRAP_OBSERVER`.** Sie schlösse die
Lücke zwischen Beitritt und Flag, verlangte aber eine Konfiguration mehr an
vier Stellen (kcl, terraform, compose, argocd-Katalog) für einen Vorgang, der
einmal passiert. Die Lücke ist Minuten lang und fällt auf, wenn sie trifft.

**Warum nicht beim Beitritt ankreuzbar.** Dann setzt es sich jeder, der nicht
in der Rangliste stehen will, und die Frage, wer mitspielt, beantwortet nicht
mehr das Büro.

**Warum kein Konto für das Zählwerk.** Naheliegend, sobald es Konten gibt, die
nicht spielen: das Zählwerk als Beobachter, und `reported_by` wäre das
Zählwerk. ADR-0015 hat das bewusst anders entschieden, und daran ändert dieses
Flag nichts:

- **Eine Maschine hat keine Identität.** Das Zählwerk weist sich über sein
  Token aus; die Herkunft steht in `entered_via = 'scoreboard'`. Beides zusammen
  sagt schon, dass eine Maschine eingetragen hat. Ein Spieler-Konto sagte es ein
  drittes Mal, ohne etwas Neues zu wissen.
- **`reported_by` benennt den Menschen, der zugesehen hat.** Stünde dort das
  Zählwerk, wäre die Frage „wer hat an diesem Abend gezählt" nicht mehr
  beantwortbar, und die Bestätigung hinge nicht mehr an einer Person.
- **Ein Konto braucht eine PIN**, und eine PIN, die in einer Maschine steht, ist
  ein zweites Token mit schwächeren Eigenschaften.

Wenn es je mehrere Zählwerke gibt und eine Zeile sagen soll, *welches*
eingetragen hat, ist die Antwort ein Token pro Gerät — wie die Kiosk-Freigaben
aus #77 —, kein Spieler.

## Konsequenzen

- **Migration, additiv:** `is_observer boolean not null default false`
  (Invariante 8). Keine bestehende Zeile ändert sich.
- **Das Repository bekommt zwei Lesarten** — alle Spieler und die, die spielen.
  Jede heutige Stelle von `Players().List` und `Players().Records` wird einer
  davon zugeordnet; Anmeldung und `/admin` lesen alle, alles andere die
  spielenden.
- **Die Regel „kein Beobachter als Spieler" sitzt in der Fachlogik**, nicht in
  den Handlern (Konventionen in `CLAUDE.md`), damit Kiosk, Turnier, Spieler-Pfad und `/api`
  sie nicht viermal schreiben.
- **`SP_BOOTSTRAP_ADMIN` fehlt im argocd-Katalog-Chart** (`apps/schmetterpause/install`),
  steht aber in kcl, terraform und compose. Ohne ihn lässt sich Schritt 2 auf
  homerun2-test1 nicht ausführen. Das ist eine eigene Lücke, die hier nur
  auffällt.

## Was ausdrücklich wartet

- **Ein Beobachter als Operator am Zählwerk.** `GET /api/players` liefert keine
  Beobachter, also bietet das Zählwerk `timoboll` nicht als Operator an. Das zu
  ändern heißt, dem Vertrag ein Feld zu geben, das das Zählwerk auswerten muss —
  eine Änderung in zwei Repositories, die erst lohnt, wenn jemand am Zählwerk
  als Beobachter zählen will.
- **Mehrere Zählwerke unterscheiden.** Siehe oben; Token pro Gerät, wenn es
  nötig wird.
