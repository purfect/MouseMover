# MouseMover

MouseMover ist eine kleine Windows-Anwendung in Go. Nach einer einstellbaren Zeit ohne **physische Mausbewegung** sendet sie in regelmäßigen Abständen zwei kleine Mausbewegungen in entgegengesetzte Richtungen. Der Mauszeiger wird bei Abweichung auf die Ausgangsposition zurückgesetzt. Alternativ kann die Anwendung über `SetThreadExecutionState` den automatischen Systemruhezustand und optional das Abschalten des Bildschirms verhindern.

**Die Anwendung garantiert keinen Anwesenheitsstatus in Teams, Slack oder ähnlichen Programmen.** Windows-Sperre, Richtlinien und andere Programme können das Verhalten beeinflussen.

## Voraussetzungen und Installation

- Windows 10 oder 11, x64, normale Benutzersitzung mit Explorer-System-Tray
- Keine Administratorrechte, Go-Installation oder zusätzliche DLLs auf dem Zielcomputer

`mm.exe` in einen dauerhaften Ordner kopieren und starten. Bei aktiviertem Autostart sollte die Datei danach nicht verschoben werden. Die Anwendung verwendet keine Netzwerkverbindung und keine Telemetrie.

Alternativ kann der MSI-Installer aus einem GitHub-Release verwendet werden. Er installiert `mm.exe` und die README unter `%LOCALAPPDATA%\Programs\MouseMover` und legt Startmenü-Einträge zum Starten und Deinstallieren an. Die Installation gilt nur für den aktuellen Benutzer und benötigt keine Administratorrechte. Autostart bleibt standardmäßig ausgeschaltet und wird ausschließlich in der Anwendung aktiviert. Vor der Deinstallation einen in der Anwendung aktivierten Autostart wieder ausschalten; die persönlichen Einstellungen unter `%APPDATA%` bleiben nach der Deinstallation erhalten.

## Bedienung

Die Inaktivitätszeit kann in Sekunden, Minuten oder Stunden eingegeben werden. Das Bewegungsintervall wird in Sekunden und die Bewegungsdistanz in Pixeln eingegeben. Standardwerte sind 60 Sekunden, 30 Sekunden und ein Pixel. Wertebereiche: 1–86400 Sekunden und 1–20 Pixel. Nach einer Änderung **Speichern** drücken. **Aktivieren** startet den gewählten Modus. Standardmäßig verschwindet dann das Fenster in den Tray. **5 Min. pausieren** unterbricht die Aktivität vorübergehend. Das Schließen des Fensters minimiert in den Tray; **Beenden** beendet die Anwendung.

Die Statusfelder zeigen Aktivierung, Pausierung, aktuelle Inaktivität, Zeit bis zur nächsten Bewegung, Bewegungszyklen, Betriebsmodus und Laufzeit. Im Modus *Keep Awake* erfolgen keine Mausbewegungen. *Kombiniert* führt beides aus. Die Option *Bildschirm aktiv halten* gilt für die beiden Modi mit Ruhezustandsschutz.

Im Tray öffnet ein Doppelklick das Fenster. Das Kontextmenü bietet Aktivieren/Deaktivieren, Pausen für 5/15/30 Minuten, Moduswahl, Einstellungen und Beenden. Grün bedeutet aktiv, Gelb pausiert und Grau deaktiviert. Die Symbole werden beim Start aus in der EXE enthaltenen Farbdaten erstellt.

## Konfiguration und Autostart

Die Einstellungen liegen in `%APPDATA%\Mousemover\config.json`. Die Datei wird beim ersten Start angelegt und bei Änderungen atomar ersetzt. Ungültige oder beschädigte Einstellungen werden gemeldet und durch Standardwerte im laufenden Programm ersetzt; die beschädigte Datei wird erst beim Speichern überschrieben.

*Mit Windows starten* setzt bzw. entfernt den Eintrag `MouseMover` unter `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`. Der Eintrag startet `mm.exe --autostart --minimized`. *Beim Start automatisch aktivieren* bestimmt unabhängig davon, ob die Funktion schon beim Programmstart eingeschaltet wird. `--minimized` öffnet direkt im Tray; bei fehlendem Explorer-Tray bleibt das Fenster sichtbar.

Ein begrenztes Log liegt in `%LOCALAPPDATA%\Mousemover\logs\mm.log`. Ab 1 MiB wird beim nächsten Start nach `mm.log.old` rotiert. Einzelne Mausbewegungen des Benutzers werden nicht protokolliert.

## Build

Go 1.26 oder neuer auf Windows x64 benötigt. Im Projektordner:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Das Skript führt Tests und `go vet` aus und erzeugt `mm.exe` im Projektstamm. Alternativ:

```powershell
go build -trimpath -ldflags='-s -w -H windowsgui' -o mm.exe ./cmd/mm
```

Der Build verwendet ausschließlich die Go-Standardbibliothek und native Windows-APIs. CGO ist nicht erforderlich.

## GitHub-Release

Der Workflow unter `.github/workflows/release.yml` wird ausgelöst, sobald ein Tag wie `v1.2.3` gepusht oder ein GitHub-Release veröffentlicht wird. Beim Tag-Push legt er das Release bei Bedarf an. Er testet und baut `mm.exe` für Windows x64 ohne Konsolenfenster, erstellt mit WiX ein MSI-Paket und hängt EXE, MSI sowie SHA-256-Dateien an das Release. Tags mit Vorabversions-Suffix wie `v1.2.3-beta` sind möglich; die MSI-Version verwendet die drei numerischen Teile. Über **Actions → Build Windows Release → Run workflow** kann ein Testbuild gestartet werden. Dessen Dateien erscheinen als Workflow-Artefakt und werden keinem Release hinzugefügt. Für den lokalen MSI-Build wird zusätzlich das .NET SDK und WiX Toolset 5.0.2 benötigt.

## Technischer Aufbau

- `cmd/mm`: Windows-Fenster, Tray-Menü, Ereignisschleife, Maus-Hook und Anwendungslaufzeit
- `internal/config`: JSON-Einstellungen und Validierung
- `internal/core`: testbare Inaktivitäts-, Pausen- und Intervalllogik
- `internal/win`: Windows-API-Aufrufe

Ein Low-Level-Maus-Hook registriert physische `WM_MOUSEMOVE`-Ereignisse. Ereignisse mit `LLMHF_INJECTED` werden für den internen Inaktivitätszähler ignoriert. `GetLastInputInfo` allein könnte diesen Unterschied nicht treffen. Die Anwendung sendet keine Klicks oder Tastatureingaben. Bei Sperrung der Windows-Sitzung wird pausiert; nach Entsperrung beginnt die Inaktivitätszeit neu. Es läuft nur eine Instanz pro Benutzersitzung.

## Grenzen und Fehlerbehebung

- Die Cursorposition wird nach dem Bewegungspaar geprüft und bei Bedarf wiederhergestellt. Ein sehr kurzer sichtbarer Sprung lässt sich bei Windows-Eingabeinjektion nicht absolut ausschließen. Gleichzeitige physische Eingabe kann diese Wiederherstellung beeinflussen.
- Manche Vollbildanwendungen, geschützte Desktops, UAC-Dialoge oder Eingabeschutzmechanismen akzeptieren die Eingabe nicht. Die Anwendung versucht keinen Schutz zu umgehen.
- Automatische Pausen für Vollbildspiele und Remote Desktop sind nicht implementiert. Bei gesperrter Sitzung wird pausiert.
- Der Ruhezustandsschutz gilt für den Windows-Thread während der Aktivierung und setzt keine administrativen Richtlinien außer Kraft.
- Wenn der Explorer-Tray beim Start fehlt, bleibt das Fenster sichtbar und die Anwendung versucht jede Sekunde erneut, ein Tray-Symbol anzulegen.
- Bei Problemen das Log prüfen. Eine beschädigte Konfiguration kann durch Speichern der angezeigten Standardwerte repariert werden.

Automatisierte Tests decken Konfigurationsvalidierung, Speichern/Laden sowie die Pausen- und Intervalllogik ab. Der Windows-Build und ein Start mit isolierten AppData-Pfaden wurden geprüft. In der aktuellen Testumgebung verweigert Windows `GetCursorPos` mit „Access is denied“, und ein Explorer-Tray ist nicht erreichbar. Die tatsächliche Tray-Bedienung, physische Mausaktivität, Registry-Änderung und das Sperren/Entsperren einer Windows-Desktop-Sitzung müssen daher in einer interaktiven Windows-Sitzung geprüft werden.
