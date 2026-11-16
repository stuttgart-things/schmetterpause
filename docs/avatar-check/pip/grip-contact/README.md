> Historischer Andrück-Stand. Aktuell: [freigegebener Produkt-Fäustling](../mitten-product/README.md).

# hank – überprüfter Griffkontakt in App und Prüfbild

## Welcher Stand war ausgeliefert?

Vorher-Prozess: PID 4030381, Binärdatei `/tmp/schmetterpause-hank`,
Startverzeichnis `/home/sthings/projects/schmetterpause`. Die App lief **bereits
im normalen Repo**; die Prüfansicht wurde aus dem separaten Worktree bedient.
Die Binärdatei enthielt den Stand bddb539, obwohl der normale Workspace inzwischen
auf main stand. Ein Branchwechsel verändert eingebettete SVGs einer laufenden
Binärdatei nicht.

Das per HTTP gelieferte Schlägerasset war bytegleich zu bddb539:
`68232cbea217efdf2051e5bc2a1f06610983af1dfe7e8e1387806bf440234bbb`.
Der bereinigte Geometriehash (Körper, neutrales Gesicht und Schläger) war ebenfalls
identisch: `c6217e37a2e71d5f0ddc3126c1d6ba80643360880004892ffcbb57d50a4a6282`.
Die vollständigen Live-/Flat-SVG-Bytehashes unterscheiden sich wegen anderer
Farben und der zusätzlichen Live-Gesichter; siehe `before-http-audit.json`.
Es wurde also keine andere Geometrieversion ausgeliefert.

## Ursache und Korrektur

Der Griff war bereits kurz nach dem ersten vollständig verdeckten Querschnitt
abgeschnitten. Nur **6.29 %** seiner
Länge lagen vollständig hinter der Flosse. Der bisherige Test prüfte lediglich,
dass der Abschluss verdeckt war; er prüfte keine Mindestüberdeckungslänge.

Der versteckte Griff läuft jetzt bis kurz vor das Ende des sicher bedeckten
Bereichs. Zusätzlich wurde die 80-%-Flosse um 0,50 nach rechts und 0,05 nach oben
verschoben. Form, Größe, Winkel, Schlägerfläche, Achse, Gesichter, Emblem und
Rahmung bleiben unverändert. Die Zeichenreihenfolge ist Griff → Flosse → Fläche.

| Messung | Vorher bddb539 | Nachher |
|---|---:|---:|
| Über volle Breite verdeckter Griffanteil | 6.29 % | 32.75 % ≥ 30 % |
| Abstand Überdeckungszentrum / Griffachse | 3.634 | 1.604 ≤ 5 |
| Flossengröße | 80 % | 80 % |
| Emblemabstand | 12,541 | 13.043 ≥ 12 |
| Canvas-Abstand | 8,070 | 8.120 ≥ 8 |
| Sichtbarer Griffhals | 20,7 | 20.20 ≥ 20 |
| Gesicht-/Flächenüberdeckung | 0 | 0 |
| Unteres sichtbares Griffende | keines | keines |

Die Vektormessung nutzt die unveränderte Achse des vollständigen Baseline-Griffs,
0,25-Einheiten-Abstände und die tatsächlichen gefüllten
Pfade einschließlich der Griffkontur. Der zusätzliche HTTP-Test rastert die
Antwort unabhängig bei 420 px: rund 33 % Vollbreitenüberdeckung und 1,6
Einheiten Mittenabweichung. Beide Methoden erfüllen die Grenzen.

## HTTP- und Pixeltests

`internal/server/hank_contact_test.go` erzeugt einen echten HTTP-Server mit
hank-Profil, lädt dessen SVG über HTTP und prüft:

- Griffachse schneidet die Flosse; Überdeckungszentrum höchstens 5 Einheiten daneben.
- Mindestens 30 % des Griffs über die komplette Breite verdeckt.
- Zeichenreihenfolge Griff → Flosse → Schlägerfläche.
- Kein Griffpfad ragt hinter dem unteren verdeckten Abschnitt heraus.
- Im aus dem HTTP-SVG gerenderten 420-px-Bild hat der innere Eintrittspunkt
  Flossenfarbe statt Grifffarbe.

Mit `SP_HANK_CONTACT_URL` wurde derselbe Test zusätzlich gegen das tatsächlich
laufende Profil auf Port 8080 ausgeführt, nicht nur gegen den Testserver.
Die 24 bestehenden Python-Geometrietests und die gesamte Go-Suite einschließlich
der bestehenden hank-/blecki-Browsertests sind grün. Unterscheidbarkeit: 186 ≥ 157.

Die echte Profilfigur und das Vergleichsblatt wurden unabhängig im Browser bei
420 × 420 an derselben Pixelposition gerendert: **0 unterschiedliche Pixel**.
PNG-SHA-256 beider Bilder: `4be9768b82257913054ea0c3574108711c5347a4d3350e913a2cfe7a8437695c`.
SVG-Geometriehash beider Quellen: `f9cd8b23852f71b0e05d4b4bc29fa8df42ae60137c22c604cec730f9f9bf9abf`.

## Laufende Quellen und Prüfbilder

App: `/home/sthings/projects/schmetterpause/.task/hank-preview/schmetterpause`, Arbeitsverzeichnis `/home/sthings/projects/schmetterpause`.
Gebaut und gestartet aus dem normalen Repo auf `feat/pip-avatar`.
Prüfansicht: normales Repo unter `/home/sthings/projects/schmetterpause`, Port 8091.
Der alte Worktree bleibt nur als losgelöste Referenz, kein Prozess nutzt ihn als Quelle.
`internal/server/zz_shot_test.go` war beim Arbeitsbeginn bereits nicht mehr vorhanden
und ist nicht Teil dieses Commits.

Für die echte Profilaufnahme wurde in der lokalen Vorschau ein eigenes Testprofil
angelegt; bestehende Spieler wurden nicht umgestellt. Profil:
`http://127.0.0.1:8080/players/07898f8e-07dd-4942-8546-600be7594252`.

- `profile-app-vs-sheet.png`: echter vollständiger Profilseiten-Screenshot neben Referenz.
- `app-vs-sheet-420.png`: echte Profilfigur neben dem Vergleichsblatt, je 420 px.
- `app-render-420.png`, `sheet-render-420.png`: pixelidentische Einzelbilder.
- `hand-before-after-420.png`, `hand-before-420.png`, `hand-after-420.png`: Handdetail
  vorher (bddb539) und nachher, je 420 px und identische rote Grifffarbe.
- `before-profile-app.png`, `after-profile-app.png`: echte Profilseite vor/nach Neustart.
- `before-http-audit.json`, `contact-audit.json`, `runtime-verification.json`:
  Quell-, Hash- und Kontaktbelege.

Reproduktion: `go run ./scripts/avatar_review.go -contact-only -out docs/avatar-check/pip/grip-contact`;
mit eigener Vorschau-Sitzung `scripts/hank_contact.browser.cjs`, anschließend
`python scripts/hank_contact_report.py`. Cache-Busting aller Bilder über den
aktuellen Asset-Inhaltshash. Physischer LED-Test ist nicht Teil der Prüfung.
