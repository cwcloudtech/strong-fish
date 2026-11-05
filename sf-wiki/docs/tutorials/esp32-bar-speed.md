---
id: esp32-bar-speed
title: Build a bar-speed sensor (ESP32)
sidebar_position: 9
---

StrongFish computes bar speed from a raw accelerometer stream, server-side, so
**any** device that can reach the API can be a bar-speed sensor — not just one
brand's. This is a maker's guide to building one from a cheap **ESP32** and an
accelerometer, with a little **MicroPython** on the board. You clip it to the
bar, it posts a set's raw motion to StrongFish, and the computed set — reps,
per-rep mean and peak velocity, velocity loss, and a predicted RPE once you have
a history — shows up in your **Fitness data**, beside anything else you track.

## How it works

```
  ESP32 + accelerometer                 StrongFish API
  ─────────────────────                 ──────────────
  read acceleration ~200 Hz
  buffer one set         ── HTTP POST ─► /v1/users/me/bar-speed/analyze
  (hold a button)           (JSON)        { movement, load, unit,
                                            sampleRateHz, samples[] }
                         ◄── the set ───   reps, velocities, RPE…
```

The board does no maths: it only streams samples. StrongFish's own engine turns
them into the set and stores it.

## Authentication

The board authenticates **as you**, with a StrongFish **API key** sent in the
`X-Api-Key` header — exactly as the app's own token does, no special device
account. Create one first and keep it on the board only; treat it like a
password. See **[API keys](./api.md#api-keys)** for how to make and revoke one.

## Hardware

- An **ESP32** dev board (any variant with Wi-Fi).
- An I²C accelerometer — an **MPU-6050** is the cheap classic; any will do if you
  adjust the register reads.
- A push button, to mark a set.
- A battery or USB power.

### Wiring

I²C for the sensor, one GPIO for the button:

```
  ESP32                         MPU-6050
  ──────                        ────────
  3V3   ──────────────────────► VCC
  GND   ──────────────────────► GND
  GPIO21 (SDA) ───────────────► SDA
  GPIO22 (SCL) ───────────────► SCL

  ESP32
  GPIO0 ───[ push button ]───► GND     (held = recording)
```

## The firmware (MicroPython)

Flash MicroPython to the ESP32 (see the
[official guide](https://docs.micropython.org/en/latest/esp32/tutorial/intro.html)),
then upload this as `main.py`. Edit the constants at the top.

```python
import network, time, ujson, urequests
from machine import Pin, I2C

# ---- configure these -------------------------------------------------------
WIFI_SSID = "your-wifi"
WIFI_PASS = "your-wifi-password"
API_BASE  = "https://api.your-strongfish.example/v1"   # include /v1, no trailing slash
API_KEY   = "sf_your_member_api_key"
MOVEMENT  = "deadlift"   # squat | bench | deadlift
LOAD_KG   = 180.0
# ---------------------------------------------------------------------------

SAMPLE_RATE_HZ = 200
PERIOD_US = 1_000_000 // SAMPLE_RATE_HZ
MAX_SAMPLES = 9000       # ~45 s, the RAM we are willing to spend

MPU = 0x68
i2c = I2C(0, scl=Pin(22), sda=Pin(21), freq=400_000)
button = Pin(0, Pin.IN, Pin.PULL_UP)

def mpu_init():
    i2c.writeto_mem(MPU, 0x6B, b"\x00")  # wake up
    i2c.writeto_mem(MPU, 0x1C, b"\x08")  # ±4 g full scale -> 8192 LSB/g

def read_accel():
    d = i2c.readfrom_mem(MPU, 0x3B, 6)   # ACCEL_XOUT_H..
    def s16(hi, lo):
        v = (hi << 8) | lo
        return v - 65536 if v > 32767 else v
    k = 9.80665 / 8192                    # LSB -> m/s²
    return (s16(d[0], d[1]) * k, s16(d[2], d[3]) * k, s16(d[4], d[5]) * k)

def connect_wifi():
    wlan = network.WLAN(network.STA_IF)
    wlan.active(True)
    if not wlan.isconnected():
        wlan.connect(WIFI_SSID, WIFI_PASS)
        while not wlan.isconnected():
            time.sleep(0.2)

def record_set():
    samples = []
    t0 = time.ticks_us()
    nxt = time.ticks_us()
    while button.value() == 0 and len(samples) < MAX_SAMPLES:
        if time.ticks_diff(time.ticks_us(), nxt) < 0:
            continue
        nxt = time.ticks_add(nxt, PERIOD_US)
        ax, ay, az = read_accel()
        t = time.ticks_diff(time.ticks_us(), t0) / 1_000_000
        samples.append({"t": t, "ax": ax, "ay": ay, "az": az})
    return samples

def post_set(samples):
    if len(samples) < 8:
        return
    body = {
        "movement": MOVEMENT, "load": LOAD_KG, "unit": "kg",
        "sampleRateHz": SAMPLE_RATE_HZ, "samples": samples,
    }
    r = urequests.post(
        API_BASE + "/users/me/bar-speed/analyze",
        headers={"Content-Type": "application/json", "X-Api-Key": API_KEY},
        data=ujson.dumps(body),
    )
    print(r.status_code, r.text)   # the computed set, or the error
    r.close()

mpu_init()
connect_wifi()
print("Ready — hold the button through a set, release to send.")
while True:
    if button.value() == 0:
        s = record_set()
        print("captured", len(s), "samples")
        try:
            post_set(s)
        except Exception as e:
            print("post failed:", e)
        while button.value() == 0:
            time.sleep_ms(10)
    time.sleep_ms(10)
```

## What comes back

The POST returns the computed set as JSON — the lift, the load, the number of
reps, each rep's mean and peak concentric velocity (m/s), and a `confidence`
between 0 and 1 (how clean the capture looked). It is stored for your account, so
it also appears in **Fitness data**, merged with any tracker you have connected,
newest set of each lift first.

## Notes and tuning

- **Sample rate.** The engine assumes the rate you send in `sampleRateHz`; 200 Hz
  is a good target. A slower sensor still works, with a wider error.
- **Orientation.** This board sends acceleration only, so the engine derives the
  vertical by tracking gravity — the sensor needs no fixed mounting. If your IMU
  also streams a quaternion, send `qw, qx, qy, qz` per sample for a cleaner
  vertical.
- **Memory.** Nine thousand samples is a lot of JSON for an ESP32; if `post_set`
  runs out of memory, lower `MAX_SAMPLES`, or stream the body in chunks.
- **Security.** The API key is your credential. Don't commit it, and revoke it
  from _API keys_ if the board is lost.
- A ready-to-flash Arduino/C++ variant of this firmware lives in the StrongFish
  repo under `hardware/esp32-barspeed/`.
