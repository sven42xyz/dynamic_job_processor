package timebackoff

import (
	"math"
	"time"
)

// here we define our func for the exponential backoff strategy

func ExponentialBackoff(attempts int) time.Duration {
	return time.Duration(math.Pow(2, float64(attempts))) * time.Second
}