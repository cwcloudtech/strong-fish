---
id: vocabulary
title: RPE, 1RM and e1RM
sidebar_position: 3
---

Three concepts appear throughout the application, and it is important to read this tutorial if you are not familiar with them.

## 1RM — your one-rep max

The heaviest single you can lift on a given movement. It is the one number you enter yourself, and everything else follows from it.

You do not have to have tested it recently, or at all. An honest estimate is enough to start with, and the app is built on the assumption that it will change: update it and every program you are running recalculates, because no weight is ever stored, only computed.

## RPE — how hard a set was

**Rate of Perceived Exertion**, on a 1–10 scale. In powerlifting it is used in
one specific sense: *how many more reps could you have done?*

| RPE | Meaning | RIR |
| --- | --- | --- |
| 10 | No reps left. A true maximum. | 0 |
| 9.5 | Maybe a rep, maybe not. | 0–1 |
| 9 | One rep left. | 1 |
| 8.5 | One certain, possibly two. | 1–2 |
| **8** | **Two reps left.** | **2** |
| 7.5 | Two certain, possibly three. | 2–3 |
| 7 | Three reps left. | 3 |
| 6 | Four reps left. | 4 |

So **RPE 8 = RIR 2**: you stopped with two reps in reserve. "5 reps @ RPE 8"
means *pick a weight you could have got 7 reps with, and do 5*.

### Why coaches program in RPE

A percentage is a promise about a day you have not had yet. RPE is an instruction about the day you are actually having: if you slept badly, "3 @ RPE 8" is a lighter bar than it was last week, and it is still the right training.

### How StrongFish turns RPE into kilos

From the **RTS/Tuchscherer chart**: a table of what fraction of your max a given number of reps at a given RPE represents. A single at RPE 10 is 100% by definition; 5 reps at RPE 8 is about 76%.

The chart, not a formula. StrongFish's importer was originally written against Epley's equation and produced loads that disagreed with the coach's own spreadsheet on 12 of 15 sets - the chart is what real programs are written from, so it is what the app computes from.

:::note Sets without an RPE
A coach can prescribe a percentage of your 1RM instead. Those are used exactly as written, because they are a deliberate choice rather than an omission.
:::

## e1RM — what a set says your max is

**Estimated 1RM**: run the chart backwards. If you did 5 reps at 100kg and rated it RPE 8, that is 76% of something - so your max on that day was about 132kg.

This is what makes RPE feedback worth logging. Each set you log becomes an estimate of your max on that day, without ever having to test one, and a rising
e1RM across a block is the evidence the block is working.

By construction a set performed exactly as prescribed produces an e1RM equal to the 1RM it was computed from. That self-consistency is the property the source spreadsheet lacked and the app was built to have.

## Competition movements

Squat, bench press and deadlift are flagged as competition movements. A variation - a Larsen press, a tempo squat, a pin bench - is programmed off the competition lift's max rather than off its own, so you do not have to test a one-rep max on every accessory you have ever done.

### How a variation finds its lift

Three things are tried, in this order:

1. **A max you recorded for that exact movement.** If you have tested your paused deadlift, your paused deadlift is loaded off that.
2. **The lift the movement is filed under** in the exercise catalog, which a coach can set or correct at any time.
3. **Its name.** "Highbar squat", "Tempo squat 3:3:0", "Paused deadlift", "RDL", "Dumbbell bench" - each says which lift it belongs to, and StrongFish reads it, whatever the capitalisation and in either language.

A few movements carry a lift's name without being loaded off it - a *goblet squat*, a *Bulgarian split squat*, a *bench pull* - and those are deliberately left alone: they stay unloaded until you record a 1M (max) for them.

If none of the three answers, the load shows as **?** and the movement is listed as needing a 1RM.
