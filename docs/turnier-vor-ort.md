# Turnier vor Ort

Ein Laptop steht an der Platte, alle anderen haben ihr Handy dabei. Der Laptop
betreibt die App und ist gleichzeitig das Gerät, an dem Ergebnisse eingetragen
werden, wenn jemand sein Handy nicht dabeihat oder keine Lust hat, erst etwas
einzurichten.

Es ist dieselbe Anwendung und dasselbe Image wie überall sonst. Der Unterschied
sind drei Umgebungsvariablen und eine zweite Compose-Datei, die sie einfordert.

## Einmalig vorbereiten

### 1. Adresse des Laptops herausfinden

```sh
task office:ip
```

Das listet die Adressen, unter denen der Laptop im Netz erreichbar ist. Gesucht
ist die Adresse aus dem Büronetz — typischerweise `192.168.x.x` oder `10.x.x.x`,
nicht `127.0.0.1` und nicht die Docker-Adresse `172.17.x.x`.

### 2. `.env` anlegen

```sh
SP_PUBLIC_BASE_URL=http://192.168.1.23:8080
SP_KIOSK_TOKEN=turnier2026
SP_SESSION_KEY=<Ausgabe von: openssl rand -hex 32>
```

Die Datei steht in `.gitignore` und darf dort auch bleiben.

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

Wer sein Handy dabeihat, scannt den Aushang, trägt beim ersten Mal seinen Namen
ein und danach nur noch Ergebnisse. Der Gegner bestätigt, erst dann zählt das
Match für die Wertung.

Am Laptop läuft der Kiosk. Dort werden Spieler ohne Gerät angelegt und
Ergebnisse für zwei beliebige Spieler eingetragen. Diese Ergebnisse gelten
sofort und brauchen keine Bestätigung: wer sie eintippt, steht neben der Platte
und hat das Match gesehen.

## Danach

```sh
task office:backup   # Datenbank in eine .sql-Datei schreiben
task office:stop     # anhalten, alle Ergebnisse bleiben erhalten
```

`task office:up` startet später mit denselben Daten weiter.

> **`task down` nicht verwenden.** Das ist der Entwicklungsbefehl und löscht das
> Datenbank-Volume mitsamt allen Ergebnissen.

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
