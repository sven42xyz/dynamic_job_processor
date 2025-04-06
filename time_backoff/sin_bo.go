package timebackoff

import (
	"math"
	"math/rand/v2"
	"time"
)

// Konstante für die Oszillation
const (
	BaseDelay      = 1 * time.Second  // Basisverzögerung
	MaxDelay       = 20 * time.Second // Maximale Verzögerung
	MinOscillation = 5
	MaxOscillation = 30
	Oscillation    = 10  // Anzahl der Schritte innerhalb einer Sinuswelle
	JitterFactor   = 0.1 // Jitter (10% der Verzögerung)
)

type SinusBackoff struct {
	Oscillation  int
	PhaseShift   float64
	JitterFactor float64
}

func NewSinusBackoff() *SinusBackoff {
	oscillation := int((int(MaxDelay / BaseDelay) + int (MaxOscillation / MinOscillation)) / 2)
	if oscillation < MinOscillation {
		oscillation = MinOscillation
	}
	if oscillation > MaxOscillation {
		oscillation = MaxOscillation
	}

	phaseShift := rand.Float64()
	jitter := rand.Float64() * float64(JitterFactor)

	return &SinusBackoff{
		Oscillation:  oscillation,
		PhaseShift:   phaseShift,
		JitterFactor: jitter,
	}
}

// Funktion, die den Sinus-Backoff mit Phasenverschiebung und Jitter berechnet
func (b *SinusBackoff) CalculateBackoff(attempt int) time.Duration {
	// Berechne den Sinuswert mit einer Phasenverschiebung
	// sinFactor := math.Sin((float64(attempt%b.Oscillation) + b.PhaseShift) * (math.Pi / float64(b.Oscillation)))
	sinFactor := math.Sin((float64(attempt)*(math.Pi/float64(b.Oscillation))) + b.PhaseShift - (math.Pi / 2)) 

	// Normalisieren: sin(x) liegt zwischen -1 und 1 → skaliere auf [baseDelay, maxDelay]
	delay := BaseDelay + time.Duration((sinFactor+1.0)*float64(MaxDelay-BaseDelay)/2.0)

	// Füge Zufallseinfluss (Jitter) hinzu, um kleine Schwankungen zu erzeugen
	jitter := time.Duration(b.JitterFactor * float64(delay)) // Zufälliger Jitter
	return delay + jitter
}
