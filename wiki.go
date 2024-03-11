package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// QueryWikidata führt eine Abfrage auf Wikidata basierend auf dem Seitentitel aus und gibt den Wappenpfad zurück.
func QueryWikidata(pageTitle, lang string) (string, error) {
	if pageTitle == "" {
		fmt.Println("Kein Titel angegeben, Abfrage wird nicht ausgeführt.")
		return "", fmt.Errorf("kein Titel angegeben")
	}

	wikidataID, err := getWikidataIDFromWikipedia(pageTitle, lang)
	if err != nil {
		fmt.Println("Fehler beim Abrufen der Wikidata ID:", err)
		return "", err
	}

	if wikidataID == "" {
		fmt.Println("Keine Wikidata ID gefunden, Abfrage wird nicht ausgeführt.")
		return "", fmt.Errorf("keine Wikidata ID gefunden")
	}

	fmt.Println("Gefundene Wikidata ID:", wikidataID)

	query := fmt.Sprintf(`SELECT ?wappen
	WHERE {
	  wd:%s wdt:P94 ?wappen. # Wappenbild
	}`, wikidataID)

	// Führe die SPARQL-Abfrage aus und gib den Wappenpfad zurück
	return sparqlQuery(query)

}

// getWikidataIDFromWikipedia ruft die Wikidata ID basierend auf dem Seitentitel ab.
func getWikidataIDFromWikipedia(pageTitle, lang string) (string, error) {
	pageTitle = strings.ReplaceAll(pageTitle, " ", "_")
	re := regexp.MustCompile(`[\(\)]`)
	pageTitle = re.ReplaceAllString(pageTitle, "")

	apiURL := fmt.Sprintf("https://%s.wikipedia.org/w/api.php", lang)
	data := url.Values{
		"action": {"query"},
		"prop":   {"pageprops"},
		"titles": {pageTitle},
		"format": {"json"},
	}

	resp, err := http.Get(apiURL + "?" + data.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	query := result["query"].(map[string]interface{})
	pages := query["pages"].(map[string]interface{})
	for _, page := range pages {
		pageData := page.(map[string]interface{})
		if pageprops, ok := pageData["pageprops"].(map[string]interface{}); ok {
			if wikibaseItem, ok := pageprops["wikibase_item"].(string); ok {
				return wikibaseItem, nil
			}
		}
	}

	return "", fmt.Errorf("Wikidata ID nicht gefunden")
}

// sparqlQuery führt die eigentliche SPARQL-Abfrage aus und gibt den Wappenpfad zurück.
func sparqlQuery(query string) (string, error) {
	endpointURL := "https://query.wikidata.org/sparql"
	client := &http.Client{}

	req, err := http.NewRequest("GET", endpointURL, nil)
	if err != nil {
		return "", fmt.Errorf("Fehler beim Erstellen der Anfrage: %v", err)
	}

	q := req.URL.Query()
	q.Add("query", query)
	q.Add("format", "json")
	req.URL.RawQuery = q.Encode()

	req.Header.Add("User-Agent", "Go_HTTP_Client/1.0")
	req.Header.Add("Accept", "application/sparql-results+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Fehler beim Senden der Anfrage: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("Fehler beim Parsen der Antwort: %v", err)
	}

	bindings := result["results"].(map[string]interface{})["bindings"].([]interface{})
	if len(bindings) > 0 {
		record := bindings[0].(map[string]interface{})
		if wappen, ok := record["wappen"].(map[string]interface{}); ok {
			if wappenURL, ok := wappen["value"].(string); ok {
				return wappenURL, nil
			}
		}
	}

	return "", fmt.Errorf("Wappen nicht gefunden")
}
