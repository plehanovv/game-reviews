# ADR 0004: Independent 1-10 game ratings

Status: accepted

- Scores are integers from 1 to 10, agreed as a product requirement. Absence of a
  rating is not a zero score. A composite primary key allows one current score per
  user/game; Go validation and a SQL CHECK enforce the range.
- Ratings reference users and games, not library entries or reviews. Rating a
  game does not require adding it to a library. Removing a library entry must not
  delete its rating. Text reviews will be a separate feature.
- PUT sets the current user's score atomically, returning 200 for creation or
  replacement. Repeating an unchanged score preserves timestamps. Concurrent
  differing writes use last-writer-wins. DELETE is idempotent and returns 204.
- GET /api/v1/me/ratings/{game_id} returns only the authenticated user's rating,
  or 404 rating_not_found if absent (also when that game ID does not exist).
  Ownership is taken from the session and included in all private queries.
- GET /api/v1/games/{game_id}/rating is public and returns game_id, count, average.
  Existing games with no scores return count=0 and average=null. Missing games
  return 404 game_not_found. Individual user identities are not in the summary.
- Calculate the aggregate from committed ratings in one SQL statement. Round the
  arithmetic mean to two decimal places. No asynchronous counter or cache is added
  before measurements justify the consistency and operational complexity.
- An index on game_id supports aggregates and foreign-key maintenance. Aggregation
  remains proportional to votes for that game; load testing may justify a later
  transactional summary table. This is not a Bayesian or popularity ranking.
- Private operations use existing cookie authentication, CSRF protection and
  no-store responses. Readiness checks the ratings schema too.

Account farming, voting abuse detection, public per-user rating visibility and
ranking rules require additional product/security decisions before public launch.
