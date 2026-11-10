---
id: velocity-based-rpe
title: Velocity-based effort
sidebar_position: 8
---

How the speed of a barbell becomes a number for how hard a set was - the science behind a "measured RPE", written independently of any one tracker or brand.

This is **velocity-based training** (VBT): training guided by how fast the bar actually moves rather than by a fixed percentage or a felt rating. VBT is the official term, used throughout StrongFish - the "VBT" column on an export, the VBT panel on the fitness page - and the rest of this page is what it means.

## The problem with asking how it felt

RPE - the rate of perceived exertion - is how most lifters log effort: a 1-to-10 scale where 10 is a maximal, nothing-left grind and the lower numbers are progressively easier. Its cousin RIR, reps in reserve, is the same information the other way round:

```
RIR = 10 - RPE
```

An RPE of 8 means roughly two reps left in the tank. The trouble is that RPE is a judgement, and judgement drifts: novices underrate hard sets, the same effort earns a different number on a good day and a bad one, and what you write down afterwards is coloured by how the set ended rather than how it ran.

Bar speed does not drift. It can be measured, and it carries most of the same information.

## The first fact: the load-velocity relationship

For a given lifter on a given exercise, the speed the bar moves on the concentric - the lifting half of the rep - is tightly tied to the load. Light loads move fast, heavy loads move slowly, and the relationship is close to linear across most of the range:

```
v = a - b * load
```

where `v` is the mean concentric velocity of the rep, `load` is a percentage of the one-rep max, and `a` and `b` are constants fitted to that lifter on that exercise. Plot a few reps at known loads and the points fall on a straight line - a load-velocity profile. Because it is so repeatable, the velocity of a single fast rep is enough to read the load back off the line to within a few percent.

Mean concentric velocity - the average speed over the whole upward range - is the usual measure rather than peak velocity, because it is steadier from rep to rep.

## The second fact: failure happens at a fixed speed

The last rep a lifter can grind out, the one just before failure, comes up at about the same velocity whatever the load. A load good for two reps crawls on the second; a load good for ten crawls on the tenth, at that same crawl. That speed is the **minimum velocity threshold** (MVT), and it is roughly constant for a given exercise and lifter:

```
MVT ~= velocity of the final possible rep   (exercise- and lifter-specific)
```

For a back squat it sits near `0.30 m/s` for many lifters, lower for a bench press, different again for a deadlift. The exact value matters less than the fact that it is stable.

## Putting them together: reps in reserve from speed

If failure is a known velocity and every rep's velocity is measured, then how far a rep sits above the MVT says how much is left - a rep far faster than the MVT has reps to spare, a rep creeping toward it is near failure. That margin is the raw material for an estimate of RIR, and from it RPE:

```
RIR ~= (v_current - v_MVT) / k        RPE = 10 - RIR
```

where `k` is the velocity lost per rep as the set fatigues, fitted per exercise. The steeper a lifter's reps fall off, the larger `k`, and the fewer reps a given speed margin buys.

## The third signal: velocity loss across the set

There is a reading that needs neither a one-rep max nor a profile: how much the bar slowed from the fastest rep of the set to the current one.

```
velocity loss = (v_fastest - v_current) / v_fastest
```

Fatigue within a set is orderly - a given percentage drop lands at a given proximity to failure for a given exercise. A 20% loss on a squat leaves most lifters several reps short of failure; pushed to 40% they are close to it. Autoregulated programs often cap a set at a velocity loss rather than a rep count for exactly this reason: it ends the set at a consistent effort instead of a consistent number.

## A worked example

Take a squat set, and suppose the lifter's profile and history give:

- MVT ~= `0.30 m/s`
- velocity lost per rep near the top end ~= `0.05 m/s`, so `k = 0.05`

The set is performed and the sensor reports its final rep at `0.45 m/s`. Then:

```
RIR ~= (0.45 - 0.30) / 0.05 = 3 reps in reserve
RPE ~= 10 - 3 = 7
```

Had the last rep come up at `0.35 m/s` instead:

```
RIR ~= (0.35 - 0.30) / 0.05 = 1
RPE ~= 9
```

The same arithmetic, run on measured speeds rather than on a feeling, is what turns a tracked set into a number.

## What accuracy demands

- **It is personal.** `a`, `b`, the MVT and `k` are the lifter's own, read from their lifts over time, not taken from a table. An uncalibrated estimate is a guess.
- **It is per exercise.** Squat, bench and deadlift each have their own profile and their own MVT; a model from one does not transfer to another.
- **Technique has to be honest.** The measure assumes the concentric is done with intent - deliberately slow reps, partial reps or a bounced start all read as the wrong velocity.
- **Grinders and ballistic lifts differ.** Where the bar accelerates and then deloads near the top, mean velocity over the whole range is more honest than peak.

## How StrongFish uses it

Two readings sit side by side, because they answer the same question with different amounts of the lifter's own history.

**What the tracker concluded.** A bar-speed tracker measures the set, works out the per-rep velocities and its own measured RPE, and StrongFish reads that finished result and charts it beside the RPE the lifter reported, one bar per rep. StrongFish never second-guesses the tracker's rep detection or its velocities.

**StrongFish's own estimate.** From the per-rep velocities alone, StrongFish also reads an effort estimate using the `RIR = (v - MVT) / k` relationship above, with population MVT and `k` values per lift (squat, bench, deadlift). It is read off the set's last rep - the effort signal - and nothing is stored: the estimate is recomputed whenever it is shown. This is the number in the "VBT" column of a training export and on the fitness page's VBT panel, and because it needs no rated history it is there from the very first recorded set. A personal history, once there is one, is the more accurate reading; the population estimate is the honest starting point.

The estimate is stateless and reusable on its own, as an endpoint that takes a movement and one rep's velocity and returns the pair:

```
POST /v1/users/me/bar-speed/effort
{ "movement": "squat", "velocity": 0.45 }
->
{ "rpe": 7, "rir": 3 }
```

Because velocity cannot read a confident effort far from failure, the estimated RPE is floored at 5 (RIR 5): a fast warm-up reads as "5 or under", not a precise low number.

**RPE or RIR.** Both scales are the same number (`RIR = 10 - RPE`). Effort is always computed and stored as RPE; a member who reads in reps-in-reserve sets the effort scale in their profile, and every effort StrongFish prints - the prescription, the logged set, this estimate - is shown as `10 - RPE` instead. It is a display choice, nothing more.
