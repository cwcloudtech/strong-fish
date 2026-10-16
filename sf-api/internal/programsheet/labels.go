package programsheet

import "fmt"

// The sheet's own words.
//
// Kept here rather than passed in from the client: the PDF is rendered by the
// API, so it cannot reach the frontend's dictionaries, and a printed sheet in
// the wrong language is worse than a small table of six words in two.
var labels = map[string]map[string]string{
	"en": {
		"date":         "Date",
		"steps":        "Steps",
		"heartAvg":     "Avg bpm",
		"heartMax":     "Max bpm",
		"zone":         "Zone",
		"totals":       "Total",
		"healthFor":    "Health data - %s",
		"stepsAvg":     "%s steps a day on average",
		"bpmNone":      "no heart rate over this range",
		"bpmRange":     "heart rate over the last %d days: %d bpm on average, peaking at %d",
		"zoneLight":    "Light",
		"zoneModerate": "Moderate",
		"zoneVigorous": "Vigorous",
		"zonePeak":     "Peak",
		"zoneNone":     "-",
		"week":         "Week %d",
		"weekless":     "Sessions",
		"day":          "Day %d",
		"rest":         "No sets in this session.",
		"exercise":     "Exercise",
		"reps":         "Reps",
		"intensity":    "Intensity",
		"load":         "Load",
		"notes":        "Notes",
		"beltless":     "beltless",
		"repsDone":     "Reps done",
		"rpeFelt":      "RPE felt",
		"e1rm":         "e1RM",
		"comment":      "Athlete's comment",
	},
	"fr": {
		"date":         "Date",
		"steps":        "Pas",
		"heartAvg":     "bpm moyen",
		"heartMax":     "bpm max",
		"zone":         "Zone",
		"totals":       "Total",
		"healthFor":    "Données santé - %s",
		"stepsAvg":     "%s pas par jour en moyenne",
		"bpmNone":      "aucune fréquence cardiaque sur cette période",
		"bpmRange":     "fréquence cardiaque sur les %d derniers jours : %d bpm en moyenne, pic à %d",
		"zoneLight":    "Légère",
		"zoneModerate": "Modérée",
		"zoneVigorous": "Intense",
		"zonePeak":     "Maximale",
		"zoneNone":     "-",
		"week":         "Semaine %d",
		"weekless":     "Séances",
		"day":          "Jour %d",
		"rest":         "Aucune série dans cette séance.",
		"exercise":     "Exercice",
		"reps":         "Répétitions",
		"intensity":    "Intensité",
		"load":         "Charge",
		"notes":        "Notes",
		"beltless":     "sans ceinture",
		"repsDone":     "Reps faites",
		"rpeFelt":      "RPE ressenti",
		"e1rm":         "1RM est.",
		"comment":      "Commentaire de l'athlète",
	},
}

// heading looks a word up, falling back to English for a locale this app does
// not carry - which prints something readable rather than a blank column.
func Heading(locale, key string) string {
	if words, ok := labels[locale]; ok {
		if word, ok := words[key]; ok {
			return word
		}
	}
	return labels["en"][key]
}

func WeekLabel(locale string, number int) string {
	// Week zero is a program whose sessions were never numbered - hand-built
	// rather than imported. "Week 0" would be a lie about the block's shape.
	if number <= 0 {
		return Heading(locale, "weekless")
	}
	return fmt.Sprintf(Heading(locale, "week"), number)
}

func DayLabel(locale string, number int) string {
	if number <= 0 {
		return Heading(locale, "day")
	}
	return fmt.Sprintf(Heading(locale, "day"), number)
}

func RestLabel(locale string) string { return Heading(locale, "rest") }
