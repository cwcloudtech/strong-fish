# AI instruct 41

## VBT / StanceFitness

Based on those documentations:
* [velocity-based-rpe](../sf-wiki/docs/tutorials/velocity-based-rpe.md)
* https://stancefitness.co/blogs/news/how-we-solved-deadlift-tracking

I'd like you to add in the exports session training in VBT column, the estimated RPE or RIR depending on the user configuration. Same on fitness data page.

Update [the documentation](../sf-wiki/docs/tutorials/velocity-based-rpe.md) as well.
Also mention the term "VBT" in the various tutorials because it's the official name (and its meaning).

Avoid to store this estimation, I'd prefer to be calculated with algorithm per movements with a reusable endpoint that takes the value as inputs.
