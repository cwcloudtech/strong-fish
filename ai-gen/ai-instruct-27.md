# AI instruct 27

## Profile

Add the gender swicth male/female (male by default).

## Strength level

### Powerlifting calculator page

* Inputs (pre-filled if the user is authenticated):
  * Sex (Male / Female)
  * Unit System (kg / lbs)
  * Division (Raw / Equipped)
  * Bodyweight
  * Squat, Bench Press, and Deadlift weights

* Outputs:
  * Combined Total (Squat + Bench + Deadlift)
  * Scores calculated concurrently for: DOTS, IPF GL Points, and Wilks (1994)
  * Percentile rank bar showing where the score sits relative to the user population.

### Gamification & badge system on profiles

* Tier Classifications (based on DOTS Score):
  * Novice (< 250 DOTS): "Iron Recruit"
  * Intermediate (250 – 349 DOTS): "Platform Contender"
  * Advanced (350 – 420 DOTS): "National Caliber"
  * Elite (421 – 499 DOTS): "Master Lifter"
  * World Class (500+ DOTS): "Titan"
* Badge Logic Engine: Design a dynamic achievement trigger system for:
  * Relative Multipliers (e.g., "2x Bodyweight Squat", "2.5x BW Deadlift", "1.5x BW Bench")
  * Milestone Total Clubs (e.g., 1000 lbs Club, 500 kg Club, 1500 lbs Club)
  * Coefficient Milestones (e.g., "300 DOTS Club", "400 DOTS Club")
  * Single-Lift Specialization (e.g., "Poverty Bench Survivor" when Squat/Deadlift dwarf Bench score).

### Deliverable

1. Provide the mathematical formulas functions for DOTS, Wilks, and IPF GL Points.
2. Provide a clean JSON API data structure for user profiles, badging metadata, and unlocked achievements.
3. Calculator screens for both web and mobile as badges
