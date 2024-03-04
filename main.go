package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

// Aktualisierte Vorlage mit einem zusätzlichen Button für den Commit
const uploadFormTmpl = `
<html>
<head>
    <title>GPX Datei-Upload</title>
</head>
<body>
    <form enctype="multipart/form-data" action="/uploads/gpx" method="post">
        <input type="file" name="gpxfile" />
        <input type="submit" value="Hochladen" />
    </form>
    <form action="/commit" method="post">
        <input type="submit" value="Commit auslösen" />
    </form>
</body>
</html>
`

// uploadForm zeigt das HTML-Formular für das Hochladen von Dateien an.
func uploadForm(w http.ResponseWriter, r *http.Request) {
	t, _ := template.New("uploadform").Parse(uploadFormTmpl)
	t.Execute(w, nil)
}

// fileUploadHandler verarbeitet das Hochladen der GPX-Datei und liest sie aus.
func fileUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Parsen der Formulardaten
		err := r.ParseMultipartForm(10 << 20) // Begrenzung auf 10 MB
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Zugriff auf die hochgeladene Datei
		file, header, err := r.FormFile("gpxfile")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Speichern der Datei in einem temporären Verzeichnis
		tempFilePath := "./temp/" + header.Filename
		if _, err := os.Stat("./temp/"); os.IsNotExist(err) {
			os.MkdirAll("./temp/", os.ModePerm)
		}
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			http.Error(w, "Fehler beim Erstellen der temporären Datei", http.StatusInternalServerError)
			return
		}
		defer tempFile.Close()
		file.Seek(0, 0) // Zurück zum Anfang der Datei
		if _, err := io.Copy(tempFile, file); err != nil {
			http.Error(w, "Fehler beim Kopieren in temporäre Datei", http.StatusInternalServerError)
			return
		}

		// Parsen der GPX-Datei und Extrahieren der gewünschten Informationen
		trackInfo, err := ExtractGPXTrackInfo(tempFilePath)
		if err != nil {
			http.Error(w, "Fehler beim Extrahieren der GPX-Daten", http.StatusInternalServerError)
			return
		}

		// Ausgabe des ersten Track-Namens, Start- und Endkoordinaten
		fmt.Fprintf(w, "Track-Name: %s\n", trackInfo.Name)
		fmt.Fprintf(w, "Start: %v, %v\n", trackInfo.StartPoint.Latitude, trackInfo.StartPoint.Longitude)
		fmt.Fprintf(w, "End: %v, %v\n", trackInfo.EndPoint.Latitude, trackInfo.EndPoint.Longitude)

		// Ausgabe der Anfangs- und Endzeit
		fmt.Fprintf(w, "Anfangszeit: %v\n", trackInfo.StartTime)
		fmt.Fprintf(w, "Endzeit: %v\n", trackInfo.EndTime)

		// Speichern oder Aktualisieren der Track-Informationen in einem aggregierten JSON-File
		jsonOutputPath := "./data/gpx_uploads.json" // Fester Dateipfad für alle Uploads
		if err := SaveOrUpdateGPXTrackInfoInJSON(trackInfo, jsonOutputPath); err != nil {
			http.Error(w, "Fehler beim Speichern/Aktualisieren der GPX-Daten als JSON", http.StatusInternalServerError)
			return
		}

		InitiateImageGeneration(trackInfo)

		// Generieren der Beschreibung
		description := generateDescription(trackInfo)

		// Speichern der GPX-Track-Informationen als Markdown
		if err := SaveGPXTrackInfoAsMarkdown(trackInfo, description); err != nil {
			log.Printf("Fehler beim Speichern der Markdown-Datei: %v", err)
			http.Error(w, "Fehler beim Speichern der Markdown-Datei", http.StatusInternalServerError)
			return
		}

		// Optional: Löschen der temporären Datei nach der Verarbeitung
		os.Remove(tempFilePath)
	} else {
		// Wenn keine POST-Anfrage, einfach das Formular anzeigen
		uploadForm(w, r)
	}

}

// commitHandler führt die notwendigen Git-Befehle aus, um Änderungen zu committen und zu pushen
func commitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Hier würden Sie die Git-Befehle ausführen
		addCmd := exec.Command("git", "add", ".")
		commitCmd := exec.Command("git", "commit", "-m", "Aktualisierte GPX-Daten")
		pushCmd := exec.Command("git", "push")

		err := addCmd.Run()
		if err != nil {
			log.Fatal("Fehler beim Ausführen von 'git add': ", err)
		}

		err = commitCmd.Run()
		if err != nil {
			log.Fatal("Fehler beim Ausführen von 'git commit': ", err)
		}

		err = pushCmd.Run()
		if err != nil {
			log.Fatal("Fehler beim Ausführen von 'git push': ", err)
		}

		fmt.Fprintf(w, "Änderungen erfolgreich committet und gepusht.")
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func main() {
	http.HandleFunc("/", uploadForm)                   // Route für das Anzeigen des Formulars
	http.HandleFunc("/uploads/gpx", fileUploadHandler) // Route für das Datei-Upload-Handling
	http.HandleFunc("/commit", commitHandler)          // Neue Route für den Commit-Button

	log.Println("Server startet auf Port :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
