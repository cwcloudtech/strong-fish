-- The exercise catalog pre-loaded with every movement appearing in the
-- reference program spreadsheet (ai-gen/assets/program.xlsx), so a coach
-- importing it never has to name a single exercise by hand and every other
-- coach gets them in their autocomplete straight away.
--
-- category/oneRmRef decide how a prescribed set is loaded:
--   squat/bench/deadlift  -> percentage and RPE prescriptions resolve against
--                            the member's 1RM for that lift (a Larsen press is
--                            programmed off the bench 1RM, a paused deadlift
--                            off the deadlift 1RM, ...)
--   accessory + oneRmRef null -> loaded in absolute kilos, or by bodyweight
--
-- aliases carry the spreadsheet's own spellings (including its "Dumbbel"
-- typo), so importing it resolves onto these rows rather than creating
-- near-duplicate exercises.

INSERT INTO exercises (data) VALUES
  ('{"slug":"squat","aliases":[],"labels":{"en":"Squat","fr":"Squat"},"category":"squat","oneRmRef":"squat","bodyweight":false,"main":true}'::jsonb),
  ('{"slug":"bench","aliases":["bench-press"],"labels":{"en":"Bench press","fr":"Développé couché"},"category":"bench","oneRmRef":"bench","bodyweight":false,"main":true}'::jsonb),
  ('{"slug":"deadlift","aliases":[],"labels":{"en":"Deadlift","fr":"Soulevé de terre"},"category":"deadlift","oneRmRef":"deadlift","bodyweight":false,"main":true}'::jsonb),

  ('{"slug":"2ct-paused-bench","aliases":["2ct-paused-bench-press"],"labels":{"en":"2CT paused bench","fr":"Développé couché pause 2 temps"},"category":"bench","oneRmRef":"bench","bodyweight":false}'::jsonb),
  ('{"slug":"larsen-2ct-paused","aliases":[],"labels":{"en":"Larsen 2CT paused","fr":"Larsen press pause 2 temps"},"category":"bench","oneRmRef":"bench","bodyweight":false}'::jsonb),
  ('{"slug":"larsen-press","aliases":["larsen"],"labels":{"en":"Larsen press","fr":"Larsen press"},"category":"bench","oneRmRef":"bench","bodyweight":false}'::jsonb),
  ('{"slug":"larsen-close-grip","aliases":[],"labels":{"en":"Larsen close grip","fr":"Larsen press prise serrée"},"category":"bench","oneRmRef":"bench","bodyweight":false}'::jsonb),
  ('{"slug":"close-grip-bench","aliases":["close-grip-bench-press"],"labels":{"en":"Close grip bench","fr":"Développé couché prise serrée"},"category":"bench","oneRmRef":"bench","bodyweight":false}'::jsonb),

  ('{"slug":"paused-deadlift","aliases":[],"labels":{"en":"Paused deadlift","fr":"Soulevé de terre avec pause"},"category":"deadlift","oneRmRef":"deadlift","bodyweight":false}'::jsonb),

  ('{"slug":"tempo-squat-3-1-3","aliases":["tempo-squat"],"labels":{"en":"Tempo squat 3:1:3","fr":"Squat tempo 3:1:3"},"category":"squat","oneRmRef":"squat","bodyweight":false}'::jsonb),
  ('{"slug":"reverse-bar-squat","aliases":[],"labels":{"en":"Reverse bar squat","fr":"Squat barre inversée"},"category":"squat","oneRmRef":"squat","bodyweight":false}'::jsonb),

  ('{"slug":"pull-ups","aliases":["pullups","pull-up"],"labels":{"en":"Pull-ups","fr":"Tractions"},"category":"accessory","oneRmRef":null,"bodyweight":true}'::jsonb),
  ('{"slug":"dips","aliases":["dip"],"labels":{"en":"Dips","fr":"Dips"},"category":"accessory","oneRmRef":null,"bodyweight":true}'::jsonb),
  ('{"slug":"lateral-raises","aliases":["lateral-raise"],"labels":{"en":"Lateral raises","fr":"Élévations latérales"},"category":"accessory","oneRmRef":null,"bodyweight":false}'::jsonb),
  ('{"slug":"strict-curl","aliases":[],"labels":{"en":"Strict curl","fr":"Curl strict"},"category":"accessory","oneRmRef":null,"bodyweight":false}'::jsonb),
  ('{"slug":"dumbbell-rowing","aliases":["dumbbel-rowing","dumbbell-row"],"labels":{"en":"Dumbbell rowing","fr":"Rowing haltère"},"category":"accessory","oneRmRef":null,"bodyweight":false}'::jsonb),
  ('{"slug":"hammer-curl","aliases":[],"labels":{"en":"Hammer curl","fr":"Curl marteau"},"category":"accessory","oneRmRef":null,"bodyweight":false}'::jsonb);
