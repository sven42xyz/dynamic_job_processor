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
	router.POST("/jobs", handlers.NewJobHandler(&jobs_mutex, &pending_jobs))

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
	jobs_mutex.Lock()
	assert.Len(t, pending_jobs, 1)
	assert.Equal(t, response["uid"], pending_jobs[0].Job.UID)
	assert.Equal(t, map[string]interface{}{"key": "value"}, pending_jobs[0].Job.Data)
	jobs_mutex.Unlock()

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

	jobs_mutex.Lock()
	assert.Len(t, pending_jobs, 2)
	assert.Equal(t, "test-uid", pending_jobs[1].Job.UID)
	jobs_mutex.Unlock()

	// Aufräumen
	jobs_mutex.Lock()
	pending_jobs = []data.PendingJob{}
	jobs_mutex.Unlock()
}

func TestCheckWritable(t *testing.T) {
	// Testfall: Ziel ist beschreibbar (Status OK)
	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsOK.Close()
	config.Config = &config.WavelyConfig{}
	config.Config.TargetSystemURL = tsOK.URL

	writable, err := external.WriteCheck("test-uid")
	assert.NoError(t, err)
	assert.True(t, writable)

	// Testfall: Ziel ist nicht beschreibbar (anderer Status)
	tsNotOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusLocked)
	}))
	defer tsNotOK.Close()
	config.Config.TargetSystemURL = tsNotOK.URL

	writable, err = external.WriteCheck("test-uid")
	assert.NoError(t, err)
	assert.False(t, writable)

	// Testfall: Ziel nicht gefunden (Status NotFound)
	tsNotFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tsNotFound.Close()
	config.Config.TargetSystemURL = tsNotFound.URL

	writable, err = external.WriteCheck("test-uid")
	assert.NoError(t, err)
	assert.False(t, writable)

	// Testfall: Fehler beim Aufruf der API
	config.Config.TargetSystemURL = "invalid-url"
	writable, err = external.WriteCheck("test-uid")
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
	config.Config = &config.WavelyConfig{}
	config.Config.TargetSystemURL = tsSuccess.URL

	err := external.WriteData("test-uid", map[string]interface{}{"key": "value"})
	assert.NoError(t, err)

	// Testfall: Fehler beim Schreiben (Status nicht 2xx)
	tsError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Write error"))
	}))
	defer tsError.Close()
	config.Config.TargetSystemURL = tsError.URL

	err = external.WriteData("test-uid", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Status: 500 Internal Server Error")
	assert.Contains(t, err.Error(), "Body: Write error")

	// Testfall: Fehler beim Serialisieren der Daten
	config.Config.TargetSystemURL = tsSuccess.URL // Verwenden Sie eine gültige URL, um den HTTP-Aufruf zu ermöglichen
	invalidData := map[string]interface{}{
		"a": func() {}, // Funktion kann nicht in JSON serialisiert werden
	}
	err = external.WriteData("test-uid", invalidData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fehler beim Serialisieren der Daten zu JSON")

	// Testfall: Fehler beim Erstellen der Anfrage
	config.Config.TargetSystemURL = "%invalid-url" // Ungültige URL
	err = external.WriteData("test-uid", map[string]interface{}{"key": "value"})
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
	pending_jobs = testJobs
	persistence.SavePendingJobs(&jobs_mutex, &pending_jobs)

	// Leeren der pendingJobs im Speicher
	pending_jobs = []data.PendingJob{}

	// Wiederherstellen der Jobs
	persistence.RestorePendingJobs(&jobs_mutex, &pending_jobs)

	// Überprüfen, ob die wiederhergestellten Jobs mit den ursprünglichen übereinstimmen
	assert.Len(t, pending_jobs, 2)
	assert.Equal(t, "job1", pending_jobs[0].Job.UID)
	assert.Equal(t, 1.0, pending_jobs[0].Job.Data["a"]) // JSON unmarshals Zahlen als float64
	assert.Equal(t, "job2", pending_jobs[1].Job.UID)
	assert.Equal(t, 2.0, pending_jobs[1].Job.Data["b"])

	// Aufräumen
	os.Remove(persistence.PersistenceFileName)

	// Testfall: Keine Datei vorhanden beim Wiederherstellen
	pending_jobs = []data.PendingJob{}
	persistence.RestorePendingJobs(&jobs_mutex, &pending_jobs)
	assert.Empty(t, pending_jobs)
}

func TestProcessJobs(t *testing.T) {
	// Aufräumen: Stellen Sie sicher, dass pendingJobs leer ist
	jobs_mutex.Lock()
	pending_jobs = []data.PendingJob{}
	jobs_mutex.Unlock()

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

	jobs_mutex.Lock()
	pending_jobs = append(pending_jobs, data.PendingJob{Job: data.Job{UID: "test-uid", Data: map[string]interface{}{"key": "value"}}, CreatedAt: time.Now()})
	jobs_mutex.Unlock()

	// Kurze Zeit warten, um die Verarbeitung zu ermöglichen
	time.Sleep(100 * time.Millisecond)

	jobs_mutex.Lock()
	assert.Empty(t, pending_jobs) // Job sollte verarbeitet und entfernt worden sein
	jobs_mutex.Unlock()
	mockClient.AssertExpectations(t)

	// Testfall: Fehler beim Überprüfen des Schreibzugriffs
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return strings.Contains(req.URL.Path, "/objects/another-uid/writable") && req.Method == http.MethodGet
	})).Return(&http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}, fmt.Errorf("API error")).Once()

	jobs_mutex.Lock()
	pending_jobs = append(pending_jobs, data.PendingJob{Job: data.Job{UID: "another-uid", Data: map[string]interface{}{"key": "value"}}, CreatedAt: time.Now()})
	jobs_mutex.Unlock()

	time.Sleep(100 * time.Millisecond)

	jobs_mutex.Lock()
	assert.Len(t, pending_jobs, 1) // Job sollte nicht entfernt worden sein
	assert.Equal(t, "another-uid", pending_jobs[0].Job.UID)
	jobs_mutex.Unlock()
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
