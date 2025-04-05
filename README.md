# 🌊 Wavely – Polling, smarter than ever.

**Smooth. Resilient. And incredibly smart.**  
Wavely is a Go-based, lightweight job runner for anyone who wants to query external systems respectfully.  
Forget stubborn polling and aggressive retry - ride the wave with us instead.

---

## Why Wavely?

**Imagine this:**

- You want to query APIs regularly – but **without getting blocked.**.
- Your IoT devices are sensitive – and **don't like request storms.**.
- Your microservice needs **load distribution**, not noise.

Wavely takes care of it.  
With **smart backoff and persistent control**.  
No broker. No third-party dependencies. 100% Go.

---

## What makes Wavely special?

| Feature                         | Description |
|--------------------------------|--------------|
| 🧠 **Sinusoidal Backoff**           | Instead of exponential delay, Wavely uses a smart sine curve per worker. |
| 🎛 **Phase Shift**             | Each job runs in its own phase. No spikes, no herds. |
| 🎲 **Built-in Jitter**        | Small random deviations prevent request collisions. |
| 🔁 **Persistence on Shutdown** | Jobs are automatically saved and resumed on restart.  |
| 🚦 **Worker Limit**            | Configurable pool for maximum control over concurrency. |
| 💡 **Zero Dependencies**       | No Redis. No RabbitMQ. No bullshit. Just Go. |

---

## Use Cases

- API clients with rate limits (e.g. Twitter, GitHub, Stripe)  
- IoT systems with cyclic data polling  
- Web scrapers that play it fair  
- Microservices that act responsibly

---
