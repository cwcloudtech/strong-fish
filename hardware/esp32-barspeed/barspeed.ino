// StrongFish bar-speed — ESP32 reference firmware
//
// A cheap accelerometer on the bar, posting raw samples to StrongFish's own
// velocity endpoint. It proves the endpoint is genuinely device-agnostic: the
// same JSON the phone sends over Bluetooth, an ESP32 sends over HTTP, and the
// server does the maths (sf-api/internal/velocity).
//
// Hardware: an ESP32 and an MPU-6050 (or any I²C accelerometer — adjust the
// read). Wiring: MPU SDA->GPIO21, SCL->GPIO22, VCC->3V3, GND->GND, and a push
// button from GPIO0 to GND to mark a set (hold to record, release to send).
//
// Auth: a StrongFish API key for the member, created in the app (Settings →
// API keys). It authenticates exactly as the app's own token does — no special
// device endpoint — and is sent as the X-Api-Key header. Keep it on the device
// only; treat it like a password.
//
// Libraries: WiFi, HTTPClient, Wire (all bundled with the ESP32 core). JSON is
// written by hand to avoid a dependency and to keep the large samples array off
// the heap twice.

#include <WiFi.h>
#include <HTTPClient.h>
#include <Wire.h>

// ---- Configure these -------------------------------------------------------
static const char* WIFI_SSID = "your-wifi";
static const char* WIFI_PASS = "your-wifi-password";
// Your StrongFish API base, including /v1. No trailing slash.
static const char* API_BASE = "https://api.your-strongfish.example/v1";
static const char* API_KEY = "sf_your_member_api_key";
// The lift this device is clipped to, and the load on the bar (kg).
static const char* MOVEMENT = "deadlift"; // squat | bench | deadlift
static const float LOAD_KG = 180.0;
// ---------------------------------------------------------------------------

static const uint8_t MPU_ADDR = 0x68;
static const int BUTTON_PIN = 0;
static const float SAMPLE_RATE_HZ = 200.0;
static const uint32_t SAMPLE_PERIOD_US = (uint32_t)(1000000.0 / SAMPLE_RATE_HZ);
// ~45 s at 200 Hz: a generous cap for one set, and all the RAM we will spend.
static const int MAX_SAMPLES = 9000;

struct Sample {
  float ax, ay, az; // m/s²
};

static Sample samples[MAX_SAMPLES];
static int sampleCount = 0;

void mpuBegin() {
  Wire.begin(21, 22);
  Wire.beginTransmission(MPU_ADDR);
  Wire.write(0x6B); // PWR_MGMT_1
  Wire.write(0x00); // wake up
  Wire.endTransmission(true);
  // ±4 g full scale (0x08): a barbell rep stays well inside it.
  Wire.beginTransmission(MPU_ADDR);
  Wire.write(0x1C); // ACCEL_CONFIG
  Wire.write(0x08);
  Wire.endTransmission(true);
}

bool mpuRead(Sample& out) {
  Wire.beginTransmission(MPU_ADDR);
  Wire.write(0x3B); // ACCEL_XOUT_H
  if (Wire.endTransmission(false) != 0) return false;
  if (Wire.requestFrom((int)MPU_ADDR, 6, true) != 6) return false;
  int16_t rx = (Wire.read() << 8) | Wire.read();
  int16_t ry = (Wire.read() << 8) | Wire.read();
  int16_t rz = (Wire.read() << 8) | Wire.read();
  // ±4 g => 8192 LSB/g; to m/s².
  const float scale = 9.80665f / 8192.0f;
  out.ax = rx * scale;
  out.ay = ry * scale;
  out.az = rz * scale;
  return true;
}

void connectWifi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  while (WiFi.status() != WL_CONNECTED) {
    delay(250);
  }
}

// recordSet samples while the button is held, returning the number captured.
int recordSet() {
  sampleCount = 0;
  uint32_t next = micros();
  while (digitalRead(BUTTON_PIN) == LOW && sampleCount < MAX_SAMPLES) {
    uint32_t now = micros();
    if ((int32_t)(now - next) < 0) continue;
    next += SAMPLE_PERIOD_US;
    if (mpuRead(samples[sampleCount])) sampleCount++;
  }
  return sampleCount;
}

// postSet streams the captured samples to the velocity endpoint. The body is
// written straight into the socket in chunks so the whole JSON is never held in
// RAM twice.
void postSet(int count) {
  if (count < 8 || WiFi.status() != WL_CONNECTED) return;

  HTTPClient http;
  String url = String(API_BASE) + "/users/me/bar-speed/analyze";
  http.begin(url);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("X-Api-Key", API_KEY);

  String head = String("{\"movement\":\"") + MOVEMENT + "\",\"load\":" + String(LOAD_KG, 1) +
                ",\"unit\":\"kg\",\"sampleRateHz\":" + String(SAMPLE_RATE_HZ, 1) + ",\"samples\":[";
  String body = head;
  body.reserve(head.length() + (size_t)count * 56 + 8);
  const float dt = 1.0f / SAMPLE_RATE_HZ;
  for (int i = 0; i < count; i++) {
    if (i) body += ',';
    body += "{\"t\":" + String(i * dt, 3) +
            ",\"ax\":" + String(samples[i].ax, 4) +
            ",\"ay\":" + String(samples[i].ay, 4) +
            ",\"az\":" + String(samples[i].az, 4) + "}";
  }
  body += "]}";

  int status = http.POST(body);
  // A real device would show the returned set (reps, velocities) on a screen;
  // here the status is enough to confirm the round trip.
  Serial.printf("POST %s -> %d\n", url.c_str(), status);
  if (status > 0) Serial.println(http.getString());
  http.end();
}

void setup() {
  Serial.begin(115200);
  pinMode(BUTTON_PIN, INPUT_PULLUP);
  mpuBegin();
  connectWifi();
  Serial.println("StrongFish bar-speed ready: hold the button for a set.");
}

void loop() {
  if (digitalRead(BUTTON_PIN) == LOW) {
    int count = recordSet();
    Serial.printf("captured %d samples\n", count);
    postSet(count);
    // Debounce the release.
    while (digitalRead(BUTTON_PIN) == LOW) delay(10);
    delay(200);
  }
  delay(10);
}
