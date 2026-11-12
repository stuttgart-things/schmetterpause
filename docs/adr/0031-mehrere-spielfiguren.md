# ADR-0031: Spieler wählen ihre Spielfigur

- **Status:** accepted
- **Datum:** 2026-11-11
- **Betrifft:** Datenmodell, Oberfläche, SVG-Assets
- **Supersedes:** ADR-0029 hinsichtlich der Beschränkung auf eine Figur und
  der für alle Figuren gemeinsamen Schmuckauswahl. Alle übrigen Regeln aus
  ADR-0029 und das GIF-Format aus ADR-0030 bleiben bestehen.
- **Bezug:** Issue #375; anschließender Auftrag, die weiteren Figuren aus dem
  Tischtennis Character Creator v4 wählbar einzubauen.

## Entscheidung

Für den Release stehen sthings und blecki zur Auswahl. blecki behält den
bestehenden gespeicherten Schlüssel `vex`. Pogo, Hank (interner Schlüssel `pip`) und Kiki werden auf den
Branches `feat/pogo-fine-tuning`, `feat/pip-fine-tuning` und
`feat/kiki-fine-tuning` weiterbearbeitet und sind im Release nicht enthalten. Die
Auswahl wird mit den Farben in `player_avatars` gespeichert. Die additive
Spalte `character` hat den Standardwert `''`, der sthings bedeutet. Bestehende
Zeilen, ausgelassene Formularfelder und unbekannte gespeicherte Figuren
bleiben dadurch bei sthings. Zurücksetzen löscht wie bisher die eigene Wahl.

Der Editor zeigt die Figuren mit Vorschaubildern. Ein Wechsel lädt zunächst
nur einen Vorschlag mit den Originalmaterialien der neuen Figur; die
Schlägerfarbe bleibt erhalten. Erst Speichern ändert die eigene Figur.
Grundfarbe, Kopfschmuck, Griff und Schläger lassen sich mit den bestehenden
Paletten ändern. Die Grundfarben dürfen je Figur eingeschränkt werden;
blecki bietet nur Original, Sand, Weiß und Schoko an; auch gespeichertes Zimt
fällt beim Lesen auf Original zurück. Neue Figuren haben zusätzlich ihre Originalgrundfarbe;
Kopfschmuck und Griff können ebenfalls auf ihre Originalfarbe zurückfallen.
Die Materialfarben für Schnauze, Augen, Ohren und ähnliche Details bleiben
fest. sthings behält seine bisherigen Farben und Schmuckstücke. blecki hat ebenfalls genau einen Schmuck-Slot: Kein Schmuck,
oder Tischtennisball. Handtuch, Kapitänsbinde, Halstuch, Mond-Anhänger und
Wolfszahn-Kette entfallen nach der Platzierungsprüfung; gespeicherte Schlüssel
fallen auf Kein Schmuck zurück. Die Listen und SVGs sind je
Figur getrennt; sthings behält seine bestehenden Stücke und seine Ebenenfolge.
bleckis Schmuck liegt nach der Basis und vor dem Gesicht.

Beim Tischtennisball erscheint zusätzlich die Ballfarbe: Weiß, Orange oder
Gelb. Die additive Spalte `ball_color` speichert diese Wahl unabhängig vom
Schmuckstück. Der Standard `''` bedeutet Weiß; fehlende oder unbekannte Werte
fallen auf Weiß zurück. Andere Schmuckstücke ignorieren die Farbe, behalten
aber eine gültige gespeicherte Wahl. Würfeln wählt bei einem Ball auch dessen
Farbe; Zurücksetzen löscht die Wahl und beginnt wieder mit Weiß. Ballfläche
und aus ihr berechneter Schatten werden gemeinsam gefärbt, ohne die Geometrie
oder die bestehenden Materialpaletten zu ändern.


Jede Figur besitzt eine eigene Basis und dieselben sechs vollständigen
Gesichter. Das Gesicht bleibt vom Spiel bestimmt. SVG und GIF, Profile,
Kopfzeile, Live-Spielstand und Zufallsfiguren verwenden denselben Katalog.
Die öffentlichen Avatar-Adressen ändern sich nicht. Zufall wählt auch die
Figur; unbekannte gespeicherte Schlüssel fallen pro Teil zurück. Auch die
entfernten Figuren Fynn und Chomp werden wie unbekannte Schlüssel behandelt:
Sie sind nicht mehr wählbar, vorhandene Auswahlen zeigen sthings. Dasselbe
gilt im Release für die drei Figuren in Bearbeitung. Die gespeicherten Werte
werden dabei nicht überschrieben, damit ihre spätere Rückkehr möglich bleibt.

## Zeichnungen und technische Grenzen

Die Tiere behalten die Proportionen und die Ansicht ihrer
Originalzeichnungen. Ihre Gesichtsteile werden herausgelöst und für die sechs
Spielausdrücke ergänzt. Die Ursprungspfade werden bei der Aufbereitung auf
`0 0 420 420` gebracht, Transformationen in die Koordinaten eingerechnet und
CSS-Klassen durch Materialfüllungen ersetzt.

Die ausgelieferten SVG-Teile enthalten weiterhin nur gefüllte Vektorformen,
keine Skripte, externen Referenzen, Stile, Striche, Masken oder Filter.
`class="sthings"` und die Gesichtsgruppen bleiben als bestehende Schnittstelle
zum Zählwerk erhalten; `data-character` kennzeichnet neue Figuren.
Die Bibliothek zum Importieren gehört nur zum Entwicklungswerkzeug, nicht
ins Anwendungs-Binary oder Container-Image.

## Konsequenzen

- Ein neues Tier braucht vollständige, aufeinander abgestimmte Zeichnungen,
  einen Katalogeintrag und Tests für SVG, Gesichtswechsel und GIF.
- Alte Nutzer müssen nichts auswählen und verändern ihr Aussehen nicht.
- Kein zusätzliches JavaScript: Auswahl und Vorschau bleiben HTMX-Formulare.
- Die Anwendung prüft Figuren-, Farb- und Schmuckschlüssel. Die Datenbank
  speichert auch unbekannte Schlüssel für einen weiterhin möglichen Rückfall.
