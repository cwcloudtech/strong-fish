---
id: esp32-bar-speed
title: Construire un capteur de vitesse de barre (ESP32)
sidebar_position: 9
---

StrongFish calcule la vitesse de barre à partir d'un flux brut d'accéléromètre,
côté serveur, de sorte que **n'importe quel** appareil capable d'atteindre l'API
peut devenir un capteur de vitesse — pas seulement celui d'une marque. Voici un
guide de maker pour en fabriquer un à partir d'un **ESP32** bon marché et d'un
accéléromètre, avec un peu de **MicroPython** sur la carte. Vous le fixez à la
barre, il envoie le mouvement brut d'une série à StrongFish, et la série
calculée — répétitions, vitesse moyenne et de pointe par répétition, perte de
vitesse, et un RPE prédit une fois que vous avez un historique — apparaît dans
vos **Données de fitness**, à côté de tout ce que vous suivez déjà.

## Comment ça marche

```
  ESP32 + accéléromètre                 API StrongFish
  ─────────────────────                 ──────────────
  lit l'accélération ~200 Hz
  met en tampon une série  ─ POST HTTP ─► /v1/users/me/bar-speed/analyze
  (on maintient un bouton)    (JSON)       { movement, load, unit,
                                             sampleRateHz, samples[] }
                           ◄── la série ──   répétitions, vitesses, RPE…
```

La carte ne fait aucun calcul : elle ne fait qu'envoyer les échantillons. C'est
le moteur de StrongFish qui en tire la série et la stocke.

## Authentification

La carte s'authentifie **en tant que vous**, avec une **clé d'API** StrongFish
envoyée dans l'en-tête `X-Api-Key` — exactement comme le fait le jeton de
l'application, sans compte spécial pour l'appareil. Créez-en une d'abord et
gardez-la seulement sur la carte ; traitez-la comme un mot de passe. Voir
**[Clés d'API](./api.md#api-keys)** pour savoir comment en créer et en révoquer
une.

## Matériel

- Une carte **ESP32** (n'importe quelle variante avec du Wi-Fi).
- Un accéléromètre I²C — un **MPU-6050** est le classique bon marché ; n'importe
  lequel convient si vous adaptez la lecture des registres.
- Un bouton poussoir, pour marquer une série.
- Une batterie ou une alimentation USB.

### Câblage

I²C pour le capteur, un GPIO pour le bouton :

```
  ESP32                         MPU-6050
  ──────                        ────────
  3V3   ──────────────────────► VCC
  GND   ──────────────────────► GND
  GPIO21 (SDA) ───────────────► SDA
  GPIO22 (SCL) ───────────────► SCL

  ESP32
  GPIO0 ───[ bouton poussoir ]───► GND     (maintenu = enregistrement)
```

## Le firmware (MicroPython)

Flashez MicroPython sur l'ESP32 (voir le
[guide officiel](https://docs.micropython.org/en/latest/esp32/tutorial/intro.html)),
puis téléversez ceci en tant que `main.py`. Modifiez les constantes en haut.

```python
import network, time, ujson, urequests
from machine import Pin, I2C

# ---- à configurer ----------------------------------------------------------
WIFI_SSID = "votre-wifi"
WIFI_PASS = "mot-de-passe-wifi"
API_BASE  = "https://api.votre-strongfish.example/v1"   # inclure /v1, sans slash final
API_KEY   = "sf_votre_cle_api_membre"
MOVEMENT  = "deadlift"   # squat | bench | deadlift
LOAD_KG   = 180.0
# ---------------------------------------------------------------------------

SAMPLE_RATE_HZ = 200
PERIOD_US = 1_000_000 // SAMPLE_RATE_HZ
MAX_SAMPLES = 9000       # ~45 s, la RAM qu'on accepte d'y consacrer

MPU = 0x68
i2c = I2C(0, scl=Pin(22), sda=Pin(21), freq=400_000)
button = Pin(0, Pin.IN, Pin.PULL_UP)

def mpu_init():
    i2c.writeto_mem(MPU, 0x6B, b"\x00")  # réveil
    i2c.writeto_mem(MPU, 0x1C, b"\x08")  # pleine échelle ±4 g -> 8192 LSB/g

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
    print(r.status_code, r.text)   # la série calculée, ou l'erreur
    r.close()

mpu_init()
connect_wifi()
print("Prêt — maintenez le bouton pendant une série, relâchez pour envoyer.")
while True:
    if button.value() == 0:
        s = record_set()
        print("capturé", len(s), "échantillons")
        try:
            post_set(s)
        except Exception as e:
            print("échec de l'envoi :", e)
        while button.value() == 0:
            time.sleep_ms(10)
    time.sleep_ms(10)
```

## Ce qui revient

Le POST renvoie la série calculée en JSON — le mouvement, la charge, le nombre
de répétitions, la vitesse concentrique moyenne et de pointe de chaque
répétition (m/s), et une `confidence` entre 0 et 1 (à quel point la capture a
semblé propre). Elle est stockée pour votre compte, elle apparaît donc aussi
dans les **Données de fitness**, fusionnée avec tout traqueur que vous avez
connecté, la série la plus récente de chaque mouvement en premier.

## Le déclencher depuis StrongFish {#trigger}

Le capteur ci-dessus enregistre tout seul et envoie le résultat. On peut aussi
faire l'inverse : laisser **StrongFish indiquer à la carte qu'une série a
commencé**, de sorte que vous appuyez sur **Démarrer** sur votre téléphone et la
carte se met à bufferiser — le mouvement et la charge arrivent avec le signal.

Dans StrongFish, ajoutez un **Déclencheur d'enregistrement** (Réglages →
_Déclencheurs d'enregistrement_, ou la feuille des capteurs sur le téléphone).
C'est le **callback** de cwcloud : un petit message JSON que le téléphone envoie
au démarrage d'une série, transporté par l'un de trois moyens — un **webhook**
(POST HTTP), un message **WebSocket**, ou une publication **MQTT**. Quand un
capteur Bluetooth est aussi connecté, le téléphone déclenche le callback _et_
enregistre en Bluetooth, les deux à la fois.

Le corps est le même quel que soit le transport :

```json
{
  "event": "bar-speed",
  "action": "start",
  "lift": "squat",
  "load": 120,
  "unit": "kg",
  "source": "strongfish",
  "firedAt": "2026-11-06T18:30:00Z"
}
```

`action` vaut `"start"` quand vous commencez la série et `"stop"` quand vous la
terminez ; `lift` vaut `squat`, `bench` ou `deadlift`. Il est envoyé depuis le
téléphone, car la carte est sur le réseau de votre salle, que seul le téléphone
peut joindre.

### Webhook (HTTP)

Pointez le déclencheur vers `http://<ip-carte>/record` et faites tourner un petit
serveur sur la carte. Un **jeton** facultatif est envoyé dans l'en-tête
`Authorization` (ou un **utilisateur/mot de passe** en authentification HTTP
basique), et vous pouvez ajouter les **en-têtes** de votre choix — pour que la
carte ignore les requêtes indésirables.

```python
import socket, ujson

def serve_trigger():
    s = socket.socket()
    s.bind(('0.0.0.0', 80))
    s.listen(1)
    while True:
        conn, _ = s.accept()
        body = conn.recv(1024).split(b'\r\n\r\n', 1)[-1]
        try:
            msg = ujson.loads(body)
            if msg.get('action') == 'start':
                start_recording(msg.get('lift'), msg.get('load'))
            else:
                stop_recording()
        except Exception:
            pass
        conn.send(b'HTTP/1.1 204 No Content\r\n\r\n')
        conn.close()
```

### MQTT

Pointez le déclencheur vers un broker (`mqtt://broker.local:1883`, ou `mqtts://`
pour le TLS), donnez-lui un **topic** et, au besoin, un **utilisateur/mot de
passe** du broker et une **QoS**. La carte s'abonne au même topic :

```python
from umqtt.simple import MQTTClient

def on_message(topic, msg):
    data = ujson.loads(msg)
    if data.get('action') == 'start':
        start_recording(data.get('lift'), data.get('load'))
    else:
        stop_recording()

client = MQTTClient('esp32', 'broker.local', 1883)
client.set_callback(on_message)
client.connect()
client.subscribe(b'strongfish/bar-speed')
while True:
    client.wait_msg()
```

### WebSocket

Pointez le déclencheur vers `ws://<ip-carte>/ws` (ou `wss://` pour le TLS). La
carte fait tourner un serveur WebSocket et lit le même JSON à chaque message.
MicroPython n'a pas de serveur WebSocket dans sa bibliothèque standard ; une
petite bibliothèque comme `MicroWebSrv2` en ajoute un — ou utilisez la voie HTTP
ou MQTT ci-dessus, qui ne demandent rien de plus.

Quel que soit le transport choisi, la carte **renvoie la série terminée** à
`/v1/users/me/bar-speed/analyze` exactement comme plus haut — le déclencheur lui
dit seulement _quand_ enregistrer, et _quel_ mouvement et _quelle_ charge y
associer.

## Notes et réglages

- **Fréquence d'échantillonnage.** Le moteur suppose la fréquence que vous
  envoyez dans `sampleRateHz` ; 200 Hz est une bonne cible. Un capteur plus lent
  fonctionne quand même, avec une erreur plus large.
- **Orientation.** Cette carte n'envoie que l'accélération, le moteur en déduit
  donc la verticale en suivant la gravité — le capteur n'a besoin d'aucun montage
  fixe. Si votre IMU envoie aussi un quaternion, transmettez `qw, qx, qy, qz` par
  échantillon pour une verticale plus propre.
- **Mémoire.** Neuf mille échantillons font beaucoup de JSON pour un ESP32 ; si
  `post_set` manque de mémoire, baissez `MAX_SAMPLES`, ou envoyez le corps par
  morceaux.
- **Sécurité.** La clé d'API est votre identifiant. Ne la commitez pas, et
  révoquez-la depuis _Clés d'API_ si la carte est perdue.
- Une variante Arduino/C++ prête à flasher de ce firmware se trouve dans le dépôt
  StrongFish sous `hardware/esp32-barspeed/`.
