# ESP32 bar speed

A reference firmware that turns a cheap ESP32 + accelerometer into a bar-speed
sensor for StrongFish. It captures a set's raw acceleration and posts it to
StrongFish's own velocity endpoint, which does the maths — the same endpoint the
phone uses over Bluetooth. No vendor, no app in the middle.

## What it proves

The endpoint `POST /v1/users/me/bar-speed/analyze` is device-agnostic: any source
that can speak HTTP and send `{movement, load, unit, sampleRateHz, samples[]}`
gets back the computed set (reps, per-rep mean/peak velocity, velocity loss, and
a predicted RPE once the member has a history). The ESP32 is just one such
source.

## Hardware

- An ESP32 dev board.
- An MPU-6050 (or any I²C accelerometer — adjust `mpuRead`).
- Wiring: `SDA→GPIO21`, `SCL→GPIO22`, `VCC→3V3`, `GND→GND`.
- A push button from `GPIO0` to `GND`: hold to record a set, release to send.

## Auth

The device authenticates as the member with a **StrongFish API key**, created in
the app (Settings → API keys), sent as the `X-Api-Key` header. There is no
special device endpoint and no extracted third-party key: it is the member's own
StrongFish credential, and it is treated exactly like the app's token. Keep it on
the device only.

## Configure and flash

Edit the constants at the top of `barspeed.ino` (`WIFI_*`, `API_BASE`, `API_KEY`,
`MOVEMENT`, `LOAD_KG`), then flash with the Arduino IDE or `arduino-cli` using the
ESP32 core. Open the serial monitor at 115200 baud: hold the button through a
set, release, and the computed set prints back.

## Notes

- Sampling is 200 Hz, the rate the engine assumes; the board states its rate in
  the payload, so a slower sensor still works (with a wider error).
- This board sends acceleration only (no orientation). The engine then derives
  the vertical by tracking gravity, so the sensor needs no fixed mounting — but
  a board that also streams a quaternion gives a cleaner vertical.
- It is a reference, kept deliberately small (hand-written JSON, one set in RAM).
  A product device would add a screen, battery management and a movement/load
  picker.
