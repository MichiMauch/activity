#!/bin/bash

# Setzen Sie den Pfad zu Ihrem lokalen Hugo-Repository
HUGO_REPO_PATH="https://github.com/MichiMauch/activitylog.git"

# Setzen Sie den Pfad zu den Ordnern, in denen Ihre MD- und JSON-Dateien gespeichert sind
MD_FILES_PATH="https://github.com/MichiMauch/activity.git/data/md"
JSON_FILES_PATH="https://github.com/MichiMauch/activity.git/data/activities"

# Ziele im Hugo-Repository
HUGO_MD_TARGET="$HUGO_REPO_PATH/content/activities"
HUGO_JSON_TARGET="$HUGO_REPO_PATH/static/map-json"

# GitHub-Einstellungen
GITHUB_USERNAME="MichiMauch"
GITHUB_REPO="https://github.com/MichiMauch/activitylog.git" # Format: Benutzername/Repo-Name
GITHUB_BRANCH="main" # oder den Branch, den Sie verwenden möchten
# WARNUNG: Für Testzwecke wird der Token hier direkt verwendet. In einer Produktionsumgebung sollten Sie eine sicherere Methode wählen.
GITHUB_PAT="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"

# Kopieren der MD- und JSON-Dateien
cp $MD_FILES_PATH/*.md $HUGO_MD_TARGET
cp $JSON_FILES_PATH/*.json $HUGO_JSON_TARGET

# Wechseln in das Hugo-Repository-Verzeichnis
cd "$HUGO_REPO_PATH"

# Git-Operationen
git config user.name "$GITHUB_USERNAME"
git config user.email "michi.mauch@gmail.com"
git add .

# Commit-Meldung mit aktuellem Datum und Uhrzeit für die Nachverfolgung
COMMIT_MESSAGE="Update der Inhalte: $(date)"
git commit -m "$COMMIT_MESSAGE"

# Pushen mit Token für die Authentifizierung
git push https://${GITHUB_USERNAME}:${GITHUB_PAT}@github.com/${GITHUB_REPO}.git $GITHUB_BRANCH

echo "Änderungen erfolgreich committet und gepusht."
