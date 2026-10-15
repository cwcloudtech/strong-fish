# AI instruct 32

## Data from fitbit/Google Health

I want pairing of fitbit/google account on profile in order to fetch and print the steps number per day with view from:
* last 24h
* last 48h
* last week (with avg per 24h)
* last month (with avg per 24h)
* last 90 days (with avg per 24h)

The pairing can be done from web or mobile app if possible.
There will be other device connections, take it into account in the data-model.

Use [Google health](https://developers.google.com/health) instead of deprecated [fitbit api](https://dev.fitbit.com). Avoid deprecated endpoints.


I want the counter or highlight on barchart to be :

* red if it's under 6000 steps
* orange if it's under 10000 steps
* light green if it's under 15000 steps
* green if it's over 15000 steps
