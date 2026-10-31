# AI instruct 36

## Health data

In withings data, if the athlete mode has been enabled and if you can detect it with the API, the bodyfat is under-estimated by 5%, add in parenthesis all the values recalculated +/-5% (%bodyfat, muscular mass, etc) following this format:

```
value (~ fixed value)
```

If you cannot detect the mode, add the fixed values anyway in parenthesis.

The coefficient is to be defined in a constant and also if you can check if the value is relevant (it is according to Gemini).
