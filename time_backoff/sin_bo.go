package timebackoff

import (
	"math"
	"math/rand/v2"
	"time"
)

// Konstante für die Oszillation
const (
	BaseDelay    = 1 * time.Second  // Basisverzögerung
	MaxDelay     = 10 * time.Second // Maximale Verzögerung
	Oscillation  = 10               // Anzahl der Schritte innerhalb einer Sinuswelle
	PhaseShift   = 1.0              // Phasenverschiebung (0 - 2*Pi)
	JitterFactor = 0.1              // Jitter (10% der Verzögerung)
)

// Funktion, die den Sinus-Backoff mit Phasenverschiebung und Jitter berechnet
func SinusBackoff(attempt int) time.Duration {
	// Berechne den Sinuswert mit einer Phasenverschiebung
	sinFactor := math.Sin((float64(attempt%Oscillation) + PhaseShift) * (math.Pi / float64(Oscillation)))

	// Normalisieren: sin(x) liegt zwischen -1 und 1 → skaliere auf [baseDelay, maxDelay]
	delay := BaseDelay + time.Duration((sinFactor+1.0)*float64(MaxDelay-BaseDelay)/2.0)

	// Füge Zufallseinfluss (Jitter) hinzu, um kleine Schwankungen zu erzeugen
	jitter := time.Duration(rand.Float64() * float64(JitterFactor) * float64(delay)) // Zufälliger Jitter
	return delay + jitter
}
