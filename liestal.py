import xml.etree.ElementTree as ET
import json


def convert_gpx_to_json(gpx_file_path):
    # XML-Datei einlesen
    tree = ET.parse(gpx_file_path)
    root = tree.getroot()

    # Namespace entfernen, falls vorhanden
    for elem in root.iter():  # Aktualisierte Methode hier
        elem.tag = elem.tag.split("}", 1)[-1]
        attribs = {}
        for name, value in elem.attrib.items():
            attribs[name.split("}", 1)[-1]] = value
        elem.attrib = attribs

    # Konvertierung in die neue Struktur
    new_structure = []
    for trkpt in root.findall(".//trkpt"):
        latitude = trkpt.attrib.get("lat")
        longitude = trkpt.attrib.get("lon")
        elevation = trkpt.find("ele").text if trkpt.find("ele") is not None else None

        new_structure.append(
            {
                "latitude": float(latitude) if latitude else None,
                "longitude": float(longitude) if longitude else None,
                "elevation": float(elevation) if elevation else None,
            }
        )

    return new_structure


# Pfad zur GPX-Datei
gpx_file_path = "1.gpx"

# Konvertierung durchführen
converted_data = convert_gpx_to_json(gpx_file_path)

# Konvertierte Daten als JSON speichern
with open("converted_data.json", "w") as json_file:
    json.dump(converted_data, json_file, indent=4)

print(
    "Konvertierung abgeschlossen. Die Daten wurden in 'converted_data.json' gespeichert."
)
