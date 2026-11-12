# AI instruct 42

## VBT / StanceFitness

I want to know if the "film and track" feature is only triggering the camera recording and still recording with the VBT sensor.

If it's the case I'd like the same feature each time I click on record (from fitness data screen or training sessions) a dropdown shoice between "film and record" or "record only".

## Sleep data

### Fitbit (google health)

Use the key `OP67-WA_19SkHyu3HBpVW1qaz1lvNZC8GWliIbidtLo` to debug and get the fitbit connection because I don't understand why I still see _No sleep recorded this week._ despite on Google health app I see records for every weeks.

Doc is here: https://developers.google.com/health/data-types/sleep

### Re-implement for Garmin

Doc is here: https://developer.garmin.com/gc-developer-program/personal/swagger-docs/

I see `/sleep-service/sleep/dailySleep` in their documentation.

Priority order : Fitbit/google health, Withings, Garmin.

If there's 404 or any error do not fail the entire screen and continue to compute the data from other providers and print the error in the troubleshoot console.
