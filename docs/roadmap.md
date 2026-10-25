# Product roadmap

Game Reviews is a game catalog and community for tracking personal libraries,
rating games, and publishing reviews. Catalog content and personal activity are
separate: administrators maintain game records; members own their library entries,
ratings, and reviews.

## Implemented foundation

- PostgreSQL catalog, ID pagination, database migrations and readiness checks.
- Username/password accounts and revocable browser sessions.
- Administrator-only game creation and a local role-management command.
- Unit and PostgreSQL integration tests; CI workflow and Docker development setup.

## Next milestones

1. Personal library: planned, playing, completed, on-hold, dropped; one entry per
   user/game; only the owner can update or remove it; paginated lists.
2. Ratings and reviews: a rating can exist without a review. Define the rating
   scale, editing/deletion rules, spoilers and public visibility before migrations.
   Enforce one current rating per user/game and explicit ownership checks.
3. Game details and discovery: descriptions, genres, platforms, release dates,
   covers, search and filters. Choose licensed metadata/image sources and stable
   identifiers for imports; identical game titles are allowed.
4. Browser UI: catalog, game page, login, profile and library; make the main
   browsing/rating/review scenario usable end to end.
5. Moderation and account lifecycle: reports, review moderation, password recovery,
   account deletion, abuse controls and session management.
6. Redis for measured caching needs and shared rate limits; Kafka for concrete
   asynchronous work such as activity feeds/notifications, using a transactional
   outbox and idempotent consumers.
7. Deployment readiness: HTTPS, least-privilege database roles, metrics, backup
   restoration tests, dependency scanning, operational documentation and releases.

Comments, subscriptions, community posts and news are later extensions, not
requirements for the first useful library-and-reviews release.

## Delivery

Finish each bounded feature with tests and a descriptive commit. Push work to
the feature branch; merge into the default branch only after reviewing the diff
and CI. Document changes to public API contracts and consequential architecture.
