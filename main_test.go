package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"djp.chapter42.de/a/config"
	"djp.chapter42.de/a/data"
	"djp.chapter42.de/a/external"
	"djp.chapter42.de/a/handlers"
	"djp.chapter42.de/a/logger"
	"djp.chapter42.de/a/persistence"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockHTTPClient ist ein Mock für den http.Client
type MockHTTPClient struct {
	mock.Mock
}

// Do ist die Mock-Implementierung für die Do-Methode des http.Clients
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func initTestLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, _ := config.Build()
	return logger
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	logger.Log = initTestLogger() // Verwenden Sie den Test-Logger
	return router
}

func TestHandleNewJob(t *testing.T) {
	router := setupRouter()
	router.POST("/jobs", handlers.NewJobHandler(&jobsMutex, &pendingJobs))

	jobData := `{"uid": "test", "data": {"key": "value"}}`
	req, _ := http.NewRequest("POST", "/jobs", bytes.NewBufferString(jobData))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusAccepted, resp.Code)
	var response map[string]string
	json.Unmarshal(resp.Body.Bytes(), &response)
	assert.NotEmpty(t, response["uid"])
	assert.Equal(t, "Job akzeptiert", response["message"])

	// Überprüfen, ob der Job zu pendingJobs hinzugefügt wurde
	jobsMutex.Lock()
	assert.Len(t, pendingJobs, 1)
	assert.Equal(t, response["uid"], pendingJobs[0].Job.UID)
	assert.Equal(t, map[string]interface{}{"key": "value"}, pendingJobs[0].Job.Data)
	jobsMutex.Unlock()

	// Test mit expliziter UID
	jobDataWithUID := `{"uid": "test-uid", "data": {"key": "value"}}`
	reqWithUID, _ := http.NewRequest("POST", "/jobs", bytes.NewBufferString(jobDataWithUID))
	reqWithUID.Header.Set("Content-Type", "application/json")
	respWithUID := httptest.NewRecorder()
	router.ServeHTTP(respWithUID, reqWithUID)

	assert.Equal(t, http.StatusAccepted, respWithUID.Code)
	var responseWithUID map[string]string
	json.Unmarshal(respWithUID.Body.Bytes(), &responseWithUID)
	assert.Equal(t, "test-uid", responseWithUID["uid"])

	jobsMutex.Lock()
	assert.Len(t, pendingJobs, 2)
	assert.Equal(t, "test-uid", pendingJobs[1].Job.UID)
	jobsMutex.Unlock()

	// Aufräumen
	jobsMutex.Lock()
	pendingJobs = []data.PendingJob{}
	jobsMutex.Unlock()
}

func TestCheckWritable(t *testing.T) {
	// Testfall: Ziel ist beschreibbar (Status OK)
	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsOK.Close()
	config.Config = &data.WavelyConfig{}
	config.Config.Current.BaseURL = tsOK.URL

	writable, err := external.WriteCheck(&data.Job{UID: "test-uid"}, &config.Config.Current)
	assert.NoError(t, err)
	assert.True(t, writable)

	// Testfall: Ziel ist nicht beschreibbar (anderer Status)
	tsNotOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusLocked)
	}))
	defer tsNotOK.Close()
	config.Config.Current.BaseURL = tsNotOK.URL

	writable, err = external.WriteCheck(&data.Job{UID: "test-uid"}, &config.Config.Current)
	assert.NoError(t, err)
	assert.False(t, writable)

	// Testfall: Ziel nicht gefunden (Status NotFound)
	tsNotFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tsNotFound.Close()
	config.Config.Current.BaseURL = tsNotFound.URL

	writable, err = external.WriteCheck(&data.Job{UID: "test-uid"}, &config.Config.Current)
	assert.NoError(t, err)
	assert.False(t, writable)

	// Testfall: Fehler beim Aufruf der API
	config.Config.Current.BaseURL = "invalid-url"
	writable, err = external.WriteCheck(&data.Job{UID: "test-uid"}, &config.Config.Current)
	assert.Error(t, err)
	assert.False(t, writable)
}

func TestWriteData(t *testing.T) {
	// Testfall: Erfolgreiches Schreiben (Status 2xx)
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer tsSuccess.Close()
	config.Config = &data.WavelyConfig{}
	config.Config.Current.BaseURL = tsSuccess.URL

	err := external.WriteData(&data.Job{UID: "test-uid"}, map[string]interface{}{"key": "value"}, &config.Config.Current)
	assert.NoError(t, err)

	// Testfall: Fehler beim Schreiben (Status nicht 2xx)
	tsError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Write error"))
	}))
	defer tsError.Close()
	config.Config.Current.BaseURL = tsError.URL

	err = external.WriteData(&data.Job{UID: "test-uid"}, map[string]interface{}{"key": "value"}, &config.Config.Current)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 500 Internal Server Error")
	assert.Contains(t, err.Error(), "Body: Write error")

	// Testfall: Fehler beim Serialisieren der Daten
	config.Config.Current.BaseURL = tsSuccess.URL // Verwenden Sie eine gültige URL, um den HTTP-Aufruf zu ermöglichen
	invalidData := map[string]interface{}{
		"a": func() {}, // Funktion kann nicht in JSON serialisiert werden
	}
	err = external.WriteData(&data.Job{UID: "test-uid"}, invalidData, &config.Config.Current)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fehler beim Serialisieren der Daten zu JSON")

	// Testfall: Fehler beim Erstellen der Anfrage
	config.Config.Current.BaseURL = "%invalid-url" // Ungültige URL
	err = external.WriteData(&data.Job{UID: "test-uid"}, map[string]interface{}{"key": "value"}, &config.Config.Current)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fehler beim Erstellen der PUT-Anfrage")
}

func TestSaveAndRestorePendingJobs(t *testing.T) {
	// Aufräumen: Stellen Sie sicher, dass die Datei nicht existiert
	os.Remove(persistence.PersistenceFileName)

	// Vorbereiten einiger Test-Jobs
	testJobs := []data.PendingJob{
		{Job: data.Job{UID: "job1", Data: map[string]interface{}{"a": 1}}, CreatedAt: time.Now()},
		{Job: data.Job{UID: "job2", Data: map[string]interface{}{"b": 2}}, CreatedAt: time.Now().Add(-time.Hour)},
	}

	// Speichern der Jobs
	pendingJobs = testJobs
	persistence.SavePendingJobs(&jobsMutex, &pendingJobs)

	// Leeren der pendingJobs im Speicher
	pendingJobs = []data.PendingJob{}

	// Wiederherstellen der Jobs
	persistence.RestorePendingJobs(&jobsMutex, &pendingJobs, &config.Config.Current)

	// Überprüfen, ob die wiederhergestellten Jobs mit den ursprünglichen übereinstimmen
	assert.Len(t, pendingJobs, 2)
	assert.Equal(t, "job1", pendingJobs[0].Job.UID)
	assert.Equal(t, 1.0, pendingJobs[0].Job.Data["a"]) // JSON unmarshals Zahlen als float64
	assert.Equal(t, "job2", pendingJobs[1].Job.UID)
	assert.Equal(t, 2.0, pendingJobs[1].Job.Data["b"])

	// Aufräumen
	os.Remove(persistence.PersistenceFileName)

	// Testfall: Keine Datei vorhanden beim Wiederherstellen
	pendingJobs = []data.PendingJob{}
	persistence.RestorePendingJobs(&jobsMutex, &pendingJobs, &config.Config.Current)
	assert.Empty(t, pendingJobs)
}

func TestProcessJobs(t *testing.T) {
	// Aufräumen: Stellen Sie sicher, dass pendingJobs leer ist
	jobsMutex.Lock()
	pendingJobs = []data.PendingJob{}
	jobsMutex.Unlock()

	// Mock-HTTP-Client erstellen
	mockClient := new(MockHTTPClient)
	httpClient = &http.Client{Transport: &mockTransport{client: mockClient}} // Verwenden Sie den Mock-Transport

	// Testfall: Erfolgreiche Verarbeitung eines Jobs
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.Path, "/objects/test-uid/writable") && req.Method == http.MethodGet
	})).Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil).Once()
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.Path, "/objects/test-uid") && req.Method == http.MethodPut
	})).Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil).Once()

	jobsMutex.Lock()
	pendingJobs = append(pendingJobs, data.PendingJob{Job: data.Job{UID: "test-uid", Data: map[string]interface{}{"key": "value"}}, CreatedAt: time.Now()})
	jobsMutex.Unlock()

	// Kurze Zeit warten, um die Verarbeitung zu ermöglichen
	time.Sleep(100 * time.Millisecond)

	jobsMutex.Lock()
	assert.Empty(t, pendingJobs) // Job sollte verarbeitet und entfernt worden sein
	jobsMutex.Unlock()
	mockClient.AssertExpectations(t)

	// Testfall: Fehler beim Überprüfen des Schreibzugriffs
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.Path, "/objects/another-uid/writable") && req.Method == http.MethodGet
	})).Return(&http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}, fmt.Errorf("API error")).Once()

	jobsMutex.Lock()
	pendingJobs = append(pendingJobs, data.PendingJob{Job: data.Job{UID: "another-uid", Data: map[string]interface{}{"key": "value"}}, CreatedAt: time.Now()})
	jobsMutex.Unlock()

	time.Sleep(100 * time.Millisecond)

	jobsMutex.Lock()
	assert.Len(t, pendingJobs, 1) // Job sollte nicht entfernt worden sein
	assert.Equal(t, "another-uid", pendingJobs[0].Job.UID)
	jobsMutex.Unlock()
	mockClient.AssertExpectations(t)
}

// Hilfsstruktur und Funktion für das Testen von HTTP-Aufrufen
type mockTransport struct {
	client *MockHTTPClient
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.client.Do(req)
}

// Initialisieren Sie den httpClient mit einem Mock-Transport für Tests
var httpClient *http.Client

func init() {
	// Verwenden Sie für Tests einen Standard-HTTP-Client, der später in den Tests gemockt werden kann
	httpClient = &http.Client{}
	http.DefaultClient = httpClient
}
