package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"djp.chapter42.de/a/config"
)

func WriteData(uid string, data map[string]interface{}) error {
	targetSystemURL := config.Config.TargetSystemURL
	if targetSystemURL == "" {
		return fmt.Errorf("target_system_url ist nicht in der Konfiguration definiert")
	}
	writeURL := fmt.Sprintf("%s/objects/%s", targetSystemURL, uid)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fehler beim Serialisieren der Daten zu JSON: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, writeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen der PUT-Anfrage: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fehler beim Senden der PUT-Anfrage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fehler beim Schreiben der Daten, Status: %s, Body: %s", resp.Status, string(bodyBytes))
	}
}
