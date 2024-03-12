package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/nfnt/resize"
	"github.com/tkrajina/gpxgo/gpx"
)

var (
	EnableChatGPTAPI = false
	EnableDALL_EAPI  = false
	defaultImagePath = "./images/default/default.png" // Pfad zum Standardbild
)

// Strukturdefinitionen
type GPXTrackInfo struct {
	Name           string     `json:"name"`
	Country        string     `json:"country"`      // Feld für das Land
	CountryCode    string     `json:"country_code"` // Feld für das Land
	State          string     `json:"state"`        // Feld für den Kanton/Bundesland
	Village        string     `json:"village"`
	EndCountry     string     `json:"endcountry"`      // Feld für das Land
	EndCountryCode string     `json:"endcountry_code"` // Feld für das Land
	EndState       string     `json:"endstate"`        // Feld für den Kanton/Bundesland
	EndVillage     string     `json:"endvillage"`
	ActivityType   string     `json:"activity_type"` // Neues Feld für den Typ der Aktivität
	Length         float64    `json:"length_km"`     // Feld für die Streckenlänge in Kilometern
	Duration       string     `json:"duration"`      // Neues Feld für die Dauer in Stunden und Minuten
	MovingTime     string     `json:"moving_time"`   // Reine Bewegungszeit in Stunden und Minuten
	TotalAscent    float64    `json:"total_ascent"`  // Gesamte aufsteigende Höhenmeter
	TotalDescent   float64    `json:"total_descent"` // Gesamte absteigende Höhenmeter
	StartTime      time.Time  `json:"start_time"`
	EndTime        time.Time  `json:"end_time"`
	StartPoint     GPXPoint   `json:"start_point"`
	EndPoint       GPXPoint   `json:"end_point"`
	Points         []GPXPoint `json:"points"`
}

type GPXPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Elevation float64 `json:"elevation"` // Neues Feld für die Höhe
}

// Extrahiert den Track-Namen, die Start-/Endkoordinaten sowie Anfangs- und Endzeit aus einer GPX-Datei
func ExtractGPXTrackInfo(filePath string) (*GPXTrackInfo, error) {
	gpxFile, err := gpx.ParseFile(filePath)
	if err != nil {
		return nil, err
	}

	if len(gpxFile.Tracks) == 0 || len(gpxFile.Tracks[0].Segments) == 0 {
		return nil, fmt.Errorf("keine Tracks im GPX-File gefunden")
	}

	track := gpxFile.Tracks[0]
	segment := track.Segments[0]
	startPoint := segment.Points[0]
	endPoint := segment.Points[len(segment.Points)-1]

	// Zeitzone laden
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Fatalf("Fehler beim Laden der Zeitzone: %v", err)
	}

	trackInfo := GPXTrackInfo{
		Name: track.Name,
		StartPoint: GPXPoint{
			Latitude:  startPoint.Latitude,
			Longitude: startPoint.Longitude,
		},
		EndPoint: GPXPoint{
			Latitude:  endPoint.Latitude,
			Longitude: endPoint.Longitude,
		},

		StartTime:    startPoint.Timestamp.In(loc), // Konvertierte Zeit
		EndTime:      endPoint.Timestamp.In(loc),   // Konvertierte Zeit
		ActivityType: track.Type,
	}

	// Innerhalb von ExtractGPXTrackInfo, nachdem StartPoint gesetzt wurde:
	country, state, village, country_code, err := GetCountryAndStateFromCoordinates(trackInfo.StartPoint.Latitude, trackInfo.StartPoint.Longitude)
	if err != nil {
		fmt.Println("Fehler beim Abrufen von Land und Kanton/Bundesland:", err)
		// Entscheiden Sie, wie Sie mit dem Fehler umgehen möchten (z.B. Fehler zurückgeben oder ignorieren)
	} else {
		trackInfo.Country = country
		trackInfo.State = state
		trackInfo.Village = village
		trackInfo.CountryCode = country_code
	}

	// Informationen für den Endpunkt abrufen
	endCountry, endState, endVillage, endCountryCode, err := GetCountryAndStateFromCoordinates(trackInfo.EndPoint.Latitude, trackInfo.EndPoint.Longitude)
	if err != nil {
		fmt.Println("Fehler beim Abrufen von Informationen für den Endpunkt:", err)
		// Entscheiden Sie, wie Sie mit dem Fehler umgehen möchten
	} else {
		trackInfo.EndCountry = endCountry
		trackInfo.EndState = endState
		trackInfo.EndVillage = endVillage
		trackInfo.EndCountryCode = endCountryCode
	}

	var points []GPXPoint
	var totalLength, totalAscent, totalDescent float64
	var previousElevation float64 = startPoint.Elevation.Value() // Initialisieren mit der Höhe des ersten Punktes

	for _, segment := range track.Segments {
		totalLength += segment.Length2D()
		for i, pt := range segment.Points {
			if i > 0 { // Ab dem zweiten Punkt
				elevationDiff := pt.Elevation.Value() - previousElevation
				if elevationDiff > 0 {
					totalAscent += elevationDiff
				} else {
					totalDescent -= elevationDiff
				}
			}
			if i%50 == 0 {
				points = append(points, GPXPoint{
					Latitude:  pt.Latitude,
					Longitude: pt.Longitude,
					Elevation: pt.Elevation.Value(),
				})
			}
			previousElevation = pt.Elevation.Value() // Aktualisieren der Höhe für den nächsten Punkt
		}
	}

	// Berechnen der Dauer
	duration := trackInfo.EndTime.Sub(trackInfo.StartTime)
	totalMinutes := int(duration.Minutes())
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	trackInfo.Duration = fmt.Sprintf("%dh %dmin", hours, minutes)

	lengthInKm := totalLength / 1000
	trackInfo.Points = points
	trackInfo.Length = lengthInKm
	trackInfo.TotalAscent = totalAscent
	trackInfo.TotalDescent = totalDescent

	// Annahme: movingDuration wurde bereits definiert
	var movingDuration time.Duration

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for i, pt := range segment.Points {
				// Überspringen des ersten Punktes im Segment, da kein vorheriger Punkt zum Vergleich existiert
				if i == 0 {
					continue
				}

				// Berechnung der Zeitdifferenz und der Distanz zum vorherigen Punkt
				timeDiff := pt.Timestamp.Sub(segment.Points[i-1].Timestamp)
				distance := segment.Points[i-1].Distance2D(&pt)

				// Verhindern der Division durch Null und Berechnung der Geschwindigkeit
				if timeDiff.Seconds() > 0 {
					speed := distance / timeDiff.Seconds()

					// Fügen Sie timeDiff zur movingDuration hinzu, wenn die Geschwindigkeit den Schwellenwert überschreitet
					if speed > 0.2 { // Geschwindigkeitsschwellenwert in m/s
						movingDuration += timeDiff
					}
				}
			}
		}
	}

	// Umwandeln der movingDuration in Stunden und Minuten für die Ausgabe
	totalMovingMinutes := int(movingDuration.Minutes())
	movingHours := totalMovingMinutes / 60
	movingMinutes := totalMovingMinutes % 60

	// Setzen der berechneten reinen Bewegungszeit in trackInfo
	trackInfo.MovingTime = fmt.Sprintf("%dh %dmin", movingHours, movingMinutes)

	return &trackInfo, nil
}

// Funktion zum Abrufen der Landes- und Kanton-/Bundeslandinformationen
func GetCountryAndStateFromCoordinates(latitude, longitude float64) (string, string, string, string, error) {
	var apiResponse struct {
		Address struct {
			Country     string `json:"country"`
			State       string `json:"state"`
			Village     string `json:"village"`
			Town        string `json:"town"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}

	requestURL := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=json&lat=%f&lon=%f", latitude, longitude)

	resp, err := http.Get(requestURL)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", err
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", "", "", "", err
	}

	// Wenn Village leer ist, aber Town einen Wert hat, nutze den Wert von Town für Village.
	if apiResponse.Address.Village == "" && apiResponse.Address.Town != "" {
		apiResponse.Address.Village = apiResponse.Address.Town
	}

	// Bearbeite das Land, um nur den ersten Namen vor dem Slash zu verwenden
	countries := strings.Split(apiResponse.Address.Country, "/")
	if len(countries) > 0 {
		apiResponse.Address.Country = countries[0] // Nur den ersten Namen verwenden
	}

	// Optional: Ausgabe von Village im Terminal
	fmt.Println("Village:", apiResponse.Address.Village)
	fmt.Println("Land:", apiResponse.Address.Country)

	return apiResponse.Address.Country, apiResponse.Address.State, apiResponse.Address.Village, apiResponse.Address.CountryCode, nil
}

// Hilfsfunktion, um Umlaute zu ersetzen und unerwünschte Zeichen zu entfernen
func SanitizeFileName(input string) string {
	// Bindestriche komplett entfernen
	input = strings.ReplaceAll(input, "-", "")

	// Umlaute ersetzen
	replacements := map[string]string{
		"Ä": "Ae", "Ö": "Oe", "Ü": "Ue",
		"ä": "ae", "ö": "oe", "ü": "ue",
		"ß": "ss",
	}
	for k, v := range replacements {
		input = strings.ReplaceAll(input, k, v)
	}

	// Unerwünschte Zeichen entfernen und Leerzeichen durch Unterstriche ersetzen
	var sb strings.Builder
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
		} else if r == ' ' {
			// Leerzeichen werden in Unterstriche umgewandelt
			sb.WriteRune('_')
		}
	}
	result := sb.String()

	result = strings.ToLower(sb.String())

	// Ersetzen doppelter Unterstriche durch einfache
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	return result

}

// Beispiel-Implementierung für generateDescription
func generateDescription(trackInfo *GPXTrackInfo) string {
	if !EnableChatGPTAPI {
		log.Println("ChatGPT API ist deaktiviert. Verwende Mock-Beschreibung.")
		return "Dies ist eine Mock-Beschreibung für Testzwecke."
	}

	apiKey := os.Getenv("CHATGPT_API_KEY") // Verwenden Sie den Namen der Umgebungsvariablen

	if apiKey == "" {
		log.Fatal("CHATGPT_API_KEY ist nicht gesetzt.")
	}

	prompt := fmt.Sprintf("Verwende keine Anführungszeichen oder Hochkommas im Beschreibungstext. Schreibe eine kurze Beschreibung, maximum 100 Wörter, für eine %s-Aktivität mit dem Titel '%s', die in %s, %s startet. Die Strecke ist %.2f km lang, mit einer Gesamtdauer von %s inklusive Pausen. Die Route hat einen Gesamtaufstieg von %.0f Metern und einen Gesamtabstieg von %.0f Metern. Basierend auf diesen Informationen, bewerte die Route mit nur einem Wort am Ende der Beschreibung: leicht, mittel oder schwer.",
		trackInfo.ActivityType, trackInfo.Name, trackInfo.Village, trackInfo.Country, trackInfo.Length, trackInfo.Duration, trackInfo.TotalAscent, trackInfo.TotalDescent)

	data := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			{
				"role":    "user",
				"content": prompt, // Dein generierter Prompt basierend auf den GPX-Daten
			},
		},
	}

	body, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Fehler beim Erstellen der Anfrage-Daten: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("Fehler beim Erstellen der Anfrage: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Fehler beim Senden der Anfrage an OpenAI: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Fehler beim Lesen der Antwort: %v", err)
	}
	var aiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		log.Fatalf("Fehler beim Parsen der Antwort: %v", err)
	}

	log.Printf("Rohantwort: %s", string(respBody))

	if len(aiResp.Choices) > 0 {
		log.Println("Generierte Beschreibung:", aiResp.Choices[0].Message.Content)
		return aiResp.Choices[0].Message.Content

	}

	return "Es konnte keine Beschreibung generiert werden."
}

func callDalleAPI(prompt string) ([]byte, error) {
	apiKey := os.Getenv("CHATGPT_API_KEY_IMAGE") // Verwenden Sie den Namen der Umgebungsvariablen
	apiURL := "https://api.openai.com/v1/images/generations"

	// Konfigurieren des Anfrage-Bodys
	data := map[string]interface{}{
		"prompt": prompt,
		"n":      1,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Marshal des Request-Bodys: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Erstellen der Anfrage: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Senden der Anfrage: %v", err)
	}
	defer resp.Body.Close()

	// Prüfen des Statuscodes der Antwort
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API-Anfrage fehlgeschlagen mit Statuscode: %d", resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Lesen der Antwort: %v", err)
	}

	return respBody, nil
}

func generateAndSaveImage(description, imageName string) {
	if !EnableDALL_EAPI {
		log.Println("DALL-E API ist deaktiviert. Überspringe Bildgenerierung.")
		copyDefaultImage(imageName)
		return
	}
	log.Println("Generiere Bild für: ", imageName)
	respBody, err := callDalleAPI(description)
	if err != nil {
		log.Fatalf("Fehler bei der Bildgenerierung: %v", err)
	}

	// Parsen der JSON-Antwort, um die URL des Bildes zu extrahieren
	var jsonResponse struct {
		Data []struct {
			Url string `json:"url"` // Angenommen, die API gibt eine URL zurück
		} `json:"data"`
	}

	err = json.Unmarshal(respBody, &jsonResponse)
	if err != nil || len(jsonResponse.Data) == 0 {
		log.Fatalf("Fehler beim Parsen der JSON-Antwort oder keine Bild-URL gefunden: %v", err)
	}

	imageUrl := jsonResponse.Data[0].Url

	// Herunterladen des Bildes von der extrahierten URL
	response, err := http.Get(imageUrl)
	if err != nil {
		log.Fatalf("Fehler beim Herunterladen des Bildes: %v", err)
	}
	defer response.Body.Close()

	imageBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("Fehler beim Lesen des Bildinhalts: %v", err)
	}

	// Laden des Bildes aus den Bytes
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatalf("Fehler beim Dekodieren des Bildes: %v", err)
	}

	// Skalieren des Bildes auf 500x500 Pixel
	newImg := resize.Resize(500, 500, img, resize.Lanczos3)

	// Öffnen einer Datei zum Speichern des skalierten Bildes
	//imageName = strings.ReplaceAll(imageName, "__", "_")          // Ersetzt doppelte Unterstriche durch einzelne
	//imageName = strings.ToLower(imageName)
	imageName = strings.ToLower(imageName)
	imageName = strings.Replace(imageName, "__", "_", -1)         // Ersetzt alle "__" durch "_"
	imagePath := fmt.Sprintf("./images/teaser/%s.png", imageName) // Verwendet den modifizierten imageName für den Pfad

	file, err := os.Create(imagePath)
	if err != nil {
		log.Fatalf("Fehler beim Erstellen der Datei: %v", err)
	}
	defer file.Close()

	// Speichern des skalierten Bildes als PNG
	err = png.Encode(file, newImg)
	if err != nil {
		log.Fatalf("Fehler beim Speichern des skalierten Bildes: %v", err)
	}
}

func formatImageName(name string) string {
	// Ersetzt doppelte Unterstriche durch einzelne und wandelt in Kleinbuchstaben um
	name = strings.ReplaceAll(name, "__", "_")
	name = strings.ToLower(name)
	return name
}

func InitiateImageGeneration(trackInfo *GPXTrackInfo) {
	description := generateDescription(trackInfo)
	imageName := SanitizeFileName(trackInfo.Name)
	GenerateAndSaveImageFromDescription(description, imageName)
}

func GenerateAndSaveImageFromDescription(description, imageName string) {
	log.Println("Starte Bildgenerierung mit Beschreibung: ", description)
	// Hinzufügen der "Das Bild soll in Pixelart sein." Präambel zur Beschreibung
	fullDescription := "Das Bild soll in fotorealistisch sein. " + description
	generateAndSaveImage(fullDescription, imageName)
}

func copyDefaultImage(imageName string) {
	// Kopieren Sie das Standardbild in den Zielordner mit dem neuen Bildnamen
	imageName = strings.ToLower(imageName)
	imageName = strings.Replace(imageName, "__", "_", -1) // Ersetzt alle "__" durch "_"
	destinationPath := fmt.Sprintf("./images/teaser/%s.png", imageName)
	err := copyFile(defaultImagePath, destinationPath)
	if err != nil {
		log.Fatalf("Fehler beim Kopieren des Standardbildes: %v", err)
	}
}

func copyFile(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		return err
	}

	return nil
}

func SaveGPXTrackInfoAsMarkdown(trackInfo *GPXTrackInfo, description string, coatOfArmsURL string, endcoatOfArmsURL string) error {
	markdownDirPath := "./data/md" // Pfad festlegen, wo die MD-Dateien gespeichert werden sollen

	// Sicherstellen, dass der Pfad existiert
	if err := os.MkdirAll(markdownDirPath, os.ModePerm); err != nil {
		return err
	}

	// Dateinamen für das Markdown-File generieren und Slug in Kleinbuchstaben umwandeln,
	// danach alle doppelten Unterstriche durch einen einzelnen ersetzen
	fileName := strings.ReplaceAll(strings.ToLower(SanitizeFileName(trackInfo.Name)), "__", "_") + ".md"
	slug := strings.ReplaceAll(strings.ToLower(SanitizeFileName(trackInfo.Name)), "__", "_")
	filePath := markdownDirPath + "/" + strings.ToLower(fileName)

	// Teile die Beschreibung in Sätze auf
	sentences := strings.Split(description, ".")

	lastSentence := ""
	restOfDescription := ""

	if len(sentences) > 1 {
		// Entferne leere Elemente, die durch das Aufteilen entstanden sein könnten
		lastSentence = strings.TrimSpace(sentences[len(sentences)-2]) + "."

		// Der Rest der Beschreibung ohne den letzten Satz
		restOfDescription = strings.Join(sentences[:len(sentences)-2], ".")
	} else if len(sentences) == 1 {
		lastSentence = strings.TrimSpace(sentences[0])
		// Da nur ein Satz vorhanden ist, bleibt der Rest der Beschreibung leer
		restOfDescription = ""
	}

	// Nun enthält `lastSentence` den letzten Satz, und `restOfDescription` den Rest der Beschreibung ohne den letzten Satz.

	// Generierte Beschreibung basierend auf den Track-Informationen
	//description := generateDescription(trackInfo, description) // Diese Funktion muss implementiert werden

	// Markdown-Inhalt mit Front Matter erstellen
	markdownContent := fmt.Sprintf(`---
slug: "%s"
title: "%s"
draft: false
type: activities
date: "%s"
country: "%s"
country_code: "%s"
state: "%s"
village: "%s"
endcountry: "%s"
endcountry_code: "%s"
endstate: "%s"
endvillage: "%s"
activity_type: "%s"
length_km: %.2f
duration: "%s"
moving_time: "%s"
total_ascent: %.0f
total_descent: %.0f
start_time: "%s"
end_time: "%s"
start_point_lat: %.5f
start_point_lon: %.5f
end_point_lat: %.5f
end_point_lon: %.5f
elevation_start: %.2f
elevation_end: %.2f
difficulty: "%s"
description: "%s"
coat_of_arms_url: "%s"
endcoat_of_arms_url: "%s"
teaser_image: ./images/teaser/%s.png
gpx_download: /gpx/%s.gpx
---`, slug, trackInfo.Name, trackInfo.StartTime.Format(time.RFC3339), trackInfo.Country, trackInfo.CountryCode, trackInfo.State, trackInfo.Village, trackInfo.EndCountry, trackInfo.EndCountryCode, trackInfo.EndState, trackInfo.EndVillage, trackInfo.ActivityType,
		trackInfo.Length, trackInfo.Duration, trackInfo.MovingTime,
		trackInfo.TotalAscent, trackInfo.TotalDescent,
		trackInfo.StartTime.Format(time.RFC3339), trackInfo.EndTime.Format(time.RFC3339),
		trackInfo.StartPoint.Latitude, trackInfo.StartPoint.Longitude,
		trackInfo.EndPoint.Latitude, trackInfo.EndPoint.Longitude,
		trackInfo.StartPoint.Elevation, trackInfo.EndPoint.Elevation,
		lastSentence, restOfDescription, coatOfArmsURL, endcoatOfArmsURL, slug, slug) // Füge die generierte Beschreibung hier ein

	/* Trackpoints hinzufügen
	for _, point := range trackInfo.Points {
		markdownContent += fmt.Sprintf("  - Latitude: %.5f\n    Longitude: %.5f\n    Elevation: %.2f\n",
			point.Latitude, point.Longitude, point.Elevation)
	} */

	// Markdown-File schreiben
	return ioutil.WriteFile(filePath, []byte(markdownContent), 0644)
}

type OpenAIResponse struct {
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// SaveOrUpdateGPXTrackInfoInJSON - Speichert oder aktualisiert GPXTrackInfo in einem aggregierten JSON-File
// und speichert zusätzlich ein separates JSON-File pro Track.
func SaveOrUpdateGPXTrackInfoInJSON(trackInfo *GPXTrackInfo, jsonFilePath string) error {
	var trackInfos []GPXTrackInfo

	// Überprüfen, ob die JSON-Datei bereits existiert
	if _, err := os.Stat(jsonFilePath); err == nil {
		// Datei existiert, bestehende Daten lesen
		jsonData, readErr := ioutil.ReadFile(jsonFilePath)
		if readErr != nil {
			return readErr
		}

		// Bestehende Daten deserialisieren
		if unmarshalErr := json.Unmarshal(jsonData, &trackInfos); unmarshalErr != nil {
			return unmarshalErr
		}
	}

	// Neue Daten hinzufügen
	trackInfos = append(trackInfos, *trackInfo)

	// Daten serialisieren für aggregiertes JSON
	updatedJSONData, marshalErr := json.MarshalIndent(trackInfos, "", "    ")
	if marshalErr != nil {
		return marshalErr
	}

	// Daten zurückschreiben in das aggregierte JSON-File
	if err := ioutil.WriteFile(jsonFilePath, updatedJSONData, 0644); err != nil {
		return err
	}

	// Zusätzlich ein separates JSON-File pro Track speichern
	// Dateinamen generieren: Trackname mit Unterstrichen anstelle von Leerzeichen
	fileName := strings.ReplaceAll(strings.ToLower(SanitizeFileName(trackInfo.Name)), "__", "_") + ".json"
	filePath := "./data/activities/" + fileName

	// Daten serialisieren für einzelnes Track-JSON
	singleTrackJSONData, marshalErr := json.MarshalIndent(trackInfo, "", "    ")
	if marshalErr != nil {
		return marshalErr
	}

	// Daten zurückschreiben in das separate Track-JSON-File
	return ioutil.WriteFile(filePath, singleTrackJSONData, 0644)
}
