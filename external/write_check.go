package external

import (
	"fmt"
	"io"
	"net/http"

	"djp.chapter42.de/a/config"
	"djp.chapter42.de/a/logger"
	"go.uber.org/zap"
)

func WriteCheck(uid string) (bool, error) {
	targetSystemURL := config.Config.GetString("target_system_url")
	if targetSystemURL == "" {
		logger.Log.Fatal("target_system_url ist nicht in der Konfiguration definiert")
		return false, nil
	}

	checkURL := fmt.Sprintf("%s/objects/%s/writable", targetSystemURL, uid)
	resp, err := http.Get(checkURL)
	if err != nil {
		return false, fmt.Errorf("fehler beim Aufruf der Schreibstatus-API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		logger.Log.Warn("Zielobjekt nicht gefunden:", zap.String("uid", uid))
		return false, nil // Objekt existiert nicht oder ist nicht auffindbar, nicht als Blockade interpretieren
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Log.Debug("Schreibstatus-API Antwort:", zap.String("status", resp.Status), zap.String("body", string(bodyBytes)))
		return false, nil // Andere Statuscodes deuten auf Blockade oder Fehler hin
	}
}
