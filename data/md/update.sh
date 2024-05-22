#!/bin/bash

# Durchlaufen aller Markdown-Dateien im aktuellen Verzeichnis
for mdfile in *.md; do
  # Verwenden von sed, um .png durch .webp im Frontmatter zu ersetzen
  sed -i'.bak' -e 's/\.png/\.webp/g' "$mdfile"

  # Entfernen der Backup-Datei, die von sed erstellt wurde (optional)
  rm "${mdfile}.bak"
done

echo "Alle Frontmatter-Referenzen wurden zu .webp geändert."
