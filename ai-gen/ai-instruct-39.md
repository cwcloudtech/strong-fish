# AI instruct 39

## Nutrition

In fitness data, I also want the following datas:
* average g of protein per day
* average ingested kcal

From Yazio or MyFitnessPal apps.

I also want the ideal protein dose per day depending of the following formula
If the user has access to body composition (through withings etc):

* min = 1.6g x {bodyweight - % of bodyfat} (ffm)
* max = 2.2g x {bodyweight - % of bodyfat} (ffm)

Put 1.6 and 2.2 in constants because it can evolve depanding on medical studies.

If the user doesn't access to body composition, guess the right value `{bodyweight - % of bodyfat}` from the BMI.

And if the height ain't filled, ask to fill it to access to this informations.

Same for the right amount of kcal it should be given by withings, if it's not, guess it from the bodycomposition, then from the BMI according to medical studies with warnings because it's estimation.

Take into account the fitbit/google health/garmin data about how much kcal are spent with activities.

For Yazio, there's an unofficial package: https://pkg.go.dev/github.com/controlado/go-yazio/pkg/yazio
But I want to avoid dependancy, it can help you build a client.
There's also this node and swagger json: https://github.com/saganos/yazio_public_api

For MyFitnessPal, there's doc api here: https://myfitnesspalapi.com/docs/
There's also a java client here: https://github.com/marcosav/myfitnesspal-api

I also want histogram per day of the week like number of steps just below heartbit and before barspeed and I also want it in the pdf/image export and of course give those data to the AI feedbacks (in chat, or fitness data) with completing the skills on how interprete it.

I want it to be usefull to help the athlete and the coach to see if the athletes needs to cut, or bulk or maintain.
