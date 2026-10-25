# ADR 0003: Private personal library

Status: accepted

## Decisions

- A library entry belongs to the authenticated user and references a catalog game.
  The public request never supplies a user ID. All store queries include user_id.
  Administrator status does not grant access to another member's personal library.
- A composite primary key (user_id, game_id) prevents duplicates under concurrent
  requests. Foreign keys prevent orphan entries; removal of a user or game cascades
  to its entries. There is no public game/account deletion API yet.
- Use PUT /api/v1/me/library/{game_id} to set the status. The response is 200 for
  both insertion and replacement. An atomic PostgreSQL upsert implements this;
  there is no separate read-then-insert race. An unchanged status preserves both
  timestamps. Concurrent differing writes use last-writer-wins; optimistic
  concurrency is not implemented.
- Allowed statuses: planned, playing, completed, on_hold, dropped. Any transition
  is allowed, including replaying a completed game. Both Go validation and a SQL
  constraint enforce the set. This is current state, not a playthrough history.
- DELETE is idempotent and returns 204 even when the current user's entry is absent.
  It never reveals whether another user has an entry for that game.
- Listing is private and paginated by ascending game ID, using limit and after_id
  consistently with the catalog. Response items contain game_id, status, created_at
  and updated_at; game details remain available through the catalog API.
- Private responses have Cache-Control: no-store and use the existing cookie/CSRF
  protections. Readiness includes the library table's required columns.

## Deferred choices

Public profiles and opt-in library sharing need an explicit visibility policy.
Ratings and reviews remain separate data: removing a library entry must not
implicitly erase a public review or rating when those features are introduced.
Filtering, alternative sorting, playtime and repeated playthroughs are later work.
