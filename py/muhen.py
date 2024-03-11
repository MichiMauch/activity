import requests


def get_wikidata_id_from_wikipedia(page_title, lang="de"):
    """
    Ermittelt die Wikidata-Q-Nummer für eine gegebene Wikipedia-Seite.
    """
    session = requests.Session()
    url = f"https://{lang}.wikipedia.org/w/api.php"
    params = {
        "action": "query",
        "prop": "pageprops",
        "titles": page_title,
        "format": "json",
    }

    response = session.get(url=url, params=params)
    data = response.json()
    page = next(iter(data["query"]["pages"].values()))
    wikidata_id = page.get("pageprops", {}).get("wikibase_item", None)
    return wikidata_id


# Finde die Wikidata Q-Nummer für "Muhen"
wikidata_id = get_wikidata_id_from_wikipedia("Aargau")
if wikidata_id:
    print(f"Gefundene Wikidata ID: {wikidata_id}")

    # Endpoint und Query definieren
    endpoint_url = "https://query.wikidata.org/sparql"
    query = f"""SELECT ?einwohnerzahl ?flaeche ?website ?koordinaten ?wappen
    WHERE {{
      wd:{wikidata_id} wdt:P1082 ?einwohnerzahl. # Einwohnerzahl
      wd:{wikidata_id} wdt:P2046 ?flaeche.       # Fläche in Quadratkilometern
      wd:{wikidata_id} wdt:P856 ?website.        # Offizielle Website
      wd:{wikidata_id} wdt:P625 ?koordinaten.    # Geografische Koordinaten
      OPTIONAL {{wd:{wikidata_id} wdt:P94 ?wappen.}} # Wappenbild (Optional)
    }}"""

    # Anfrage senden
    headers = {
        "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:52.0) Gecko/20100101 Firefox/52.0",
        "Accept": "application/sparql-results+json",
    }
    response = requests.get(
        endpoint_url, headers=headers, params={"query": query, "format": "json"}
    )
    data = response.json()

    # Ergebnisse verarbeiten
    for item in data["results"]["bindings"]:
        einwohnerzahl = item.get("einwohnerzahl", {}).get("value", "n/a")
        flaeche = item.get("flaeche", {}).get("value", "n/a")
        website = item.get("website", {}).get("value", "n/a")
        koordinaten = item.get("koordinaten", {}).get("value", "n/a")
        wappen = item.get("wappen", {}).get("value", "n/a")

        print(f"Einwohnerzahl: {einwohnerzahl}")
        print(f"Fläche: {flaeche}")
        print(f"Website: {website}")
        print(f"Koordinaten: {koordinaten}")
        print(f"Wappenbild-URL: {wappen}")
        print("-------")
else:
    print("Wikidata ID konnte nicht gefunden werden.")
