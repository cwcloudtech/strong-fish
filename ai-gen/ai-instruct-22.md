# AI instruct 22

For exercize without 1RM recorded by the user, try to pick the 1RM of the comp exercize which match the most.

For example:
* Highbar squat -> squat
* Tempo squat 3:3:0 -> squat
* Paused deadlift -> deadlift
* Dumbbel bench -> bench

And there's also those matching rules to keep (I want a decision table in the backend to be updated easily):
* RDL -> deadlift

Match are case insensitive.
As upload of program (it has to check if there's already the same exercice exists case insensitive).
