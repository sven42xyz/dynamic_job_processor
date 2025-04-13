package external

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"djp.chapter42.de/a/internal/data"
	"djp.chapter42.de/a/internal/logger"
	"djp.chapter42.de/a/internal/tmpl"
	"go.uber.org/zap"
)

func WriteCheck(job *data.Job, currentCfg *data.CurrentConfig) (bool, error) {
	checkURL, err := urlBuilder(currentCfg, job, "check")
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest(http.MethodGet, checkURL, nil)
	if err != nil {
		logger.Log.Warn("Error while generating request:", zap.Error(err))
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Wavely/1.0")

	if currentCfg.AuthProvider != nil {
		auth_header, err := currentCfg.AuthProvider.GetAuthHeader()
		if err != nil {
			logger.Log.Warn("Error while generating AuthHeaders:", zap.Error(err))
			return false, err
		}
		req.Header.Set("Authorization", auth_header)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		logger.Log.Warn("Zielobjekt nicht gefunden:", zap.String("uid", job.UID))
		return false, nil // Objekt existiert nicht oder ist nicht auffindbar, nicht als Blockade interpretieren
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Log.Debug("Schreibstatus-API Antwort:", zap.String("status", resp.Status), zap.String("body", string(bodyBytes)))
		return false, nil // Andere Statuscodes deuten auf Blockade oder Fehler hin
	}
}

func WriteData(job *data.Job, data string, currentCfg *data.CurrentConfig) error {
	checkURL, err := urlBuilder(currentCfg, job, "check")
	if err != nil {
		return err
	}

	payload, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		logger.Log.Error("Error while serializing the data:", zap.Error(err))
		return err
	}

	var contentType string
	switch job.ContentType {
	case "json":
		contentType = "application/json"
	case "xml":
		contentType = "application/xml"
	}

	req, err := http.NewRequest(http.MethodPut, checkURL, bytes.NewReader(payload))
	if err != nil {
		logger.Log.Error("Error while generating request:", zap.Error(err))
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Wavely/1.0")

	if currentCfg.AuthProvider != nil {
		auth_header, err := currentCfg.AuthProvider.GetAuthHeader()
		if err != nil {
			logger.Log.Warn("Error while generating AuthHeaders:", zap.Error(err))
			return err
		}
		req.Header.Set("Authorization", auth_header)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Warn("Error while calling the write api:", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Log.Error("Error while writing data:", zap.String("Status", resp.Status), zap.String("Body", string(bodyBytes)))
		return fmt.Errorf("error while writing data %s, Body: %s", resp.Status, string(bodyBytes))
	}
}

func LatestRevision(job *data.Job, currentCfg *data.CurrentConfig) (string, error) {
	revisionURL, err := urlBuilder(currentCfg, job, "revision")
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, revisionURL, nil)
	if err != nil {
		logger.Log.Warn("Error while generating request:", zap.Error(err))
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Wavely/1.0")

	if currentCfg.AuthProvider != nil {
		auth_header, err := currentCfg.AuthProvider.GetAuthHeader()
		if err != nil {
			logger.Log.Warn("Error while generating AuthHeaders:", zap.Error(err))
			return "", err
		}
		req.Header.Set("Authorization", auth_header)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var latestRevision data.Revision
		json.Unmarshal(body, &latestRevision)
		return latestRevision.LatestRevision, nil
	} else if resp.StatusCode == http.StatusNotFound {
		logger.Log.Warn("Zielobjekt nicht gefunden:", zap.String("uid", job.UID))
		return "", nil // Objekt existiert nicht oder ist nicht auffindbar, nicht als Blockade interpretieren
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Log.Debug("Schreibstatus-API Antwort:", zap.String("status", resp.Status), zap.String("body", string(bodyBytes)))
		return "", nil // Andere Statuscodes deuten auf Blockade oder Fehler hin
	}
}

func urlBuilder(currentCfg *data.CurrentConfig, job *data.Job, ep string) (string, error) {
	var err error
	var endpoint string

	baseURL := currentCfg.BaseURL
	if baseURL == "" {
		logger.Log.Fatal("baseURL ist nicht in der Konfiguration definiert")
		return "", nil
	}

	switch ep {
	case "check":
		endpoint, err = tmpl.RenderEndpoint(currentCfg.ParsedCheckTpl, *job)
	case "write":
		endpoint, err = tmpl.RenderEndpoint(currentCfg.ParsedWriteTpl, *job)
	default:
		logger.Log.Warn("Undefined endpoint:", zap.String("Endpoint", ep), zap.Error(err))
		return "", err
	}
	if err != nil {
		logger.Log.Warn("Fehler beim Rendern des Endpunktes:", zap.Error(err))
		return "", err
	}

	fullURL := baseURL + endpoint

	return fullURL, nil
}