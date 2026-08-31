-- The hand-picked "specialty" badge is gone.
--
-- It asked a member to declare which lift was theirs, and the app then drew a
-- chip saying so. What replaced it is derived from the lifts themselves (see
-- the API's internal/strength): a squat that outweighs the deadlift, a bench
-- under a quarter of the other two, and so on. Two badges answering the same
-- question - one claimed, one measured - is one too many, and the claimed one
-- could sit on a profile contradicting the numbers underneath it.
--
-- The key is dropped rather than left in place: a payload field nothing reads
-- is a trap for the next person to grep for it, and the merge that writes a
-- profile is shallow, so an untouched key would survive every future save.
UPDATE users
SET data = data - 'specialty'
WHERE data ? 'specialty';
