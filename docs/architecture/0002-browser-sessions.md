# ADR 0002: Browser accounts and revocable sessions

Status: accepted for the current single-instance application

## Decisions

- Local username/password accounts; no email collection until verification and
  recovery flows are designed. Registration always creates the `user` role.
- Argon2id: 19 MiB, two iterations, parallelism one, independent random 16-byte
  salts. These parameters follow the OWASP baseline. PHC-style encoded hashes
  retain their algorithm and parameters. Verification accepts only the supported
  bounded profile; future upgrades must deliberately support old profiles and rehash.
- Passwords: 15-128 Unicode characters without trimming. Usernames normalize to
  lowercase ASCII. A unique PostgreSQL constraint resolves concurrent registration.
- Login creates a cryptographically random 32-byte token. Store only its SHA-256
  digest, user reference and absolute expiry in PostgreSQL. The token is returned
  only in a cookie; it is never returned in JSON or written to application logs.
- Session lifetime is 24 hours without sliding renewal. A login replaces and
  revokes the current browser session; other devices stay signed in. Logout is
  idempotent and deletes the current session. Expired tokens fail authentication
  even if their records have not been removed.
- Use HttpOnly, SameSite=Strict, Path=/, and Secure cookies. In HTTPS mode the
  `__Host-` prefix prevents Domain/path overrides. Local HTTP uses a different
  cookie name and requires explicit `COOKIE_SECURE=false`.
- Use Go net/http CrossOriginProtection for unsafe browser requests, with no
  trusted-origin exceptions. No state changes on GET. This assumes same-origin
  frontend deployment; reverse proxies must preserve the public Host and use TLS.
- Limit credential bodies to 4 KiB, reject unknown fields and extra JSON values,
  and prevent caching of account responses. Missing users and wrong passwords
  return the same login error; a dummy Argon2 hash handles nonexistent accounts.
- A bounded per-IP limiter allows 10 login/registration attempts per minute.
  At most four Argon2 operations execute concurrently per service instance.

## Boundaries and remaining deployment work

The IP limiter is in-memory and resets at restart. A reverse proxy is not yet
trusted; behind one, clients may share a limit. Multi-instance deployment needs
shared rate limits (Redis) and an explicit trusted-proxy configuration. Distributed
abuse and account-targeted throttling require additional controls.

Unique username errors intentionally reveal availability, as usernames are public.
Password reset, compromised-password screening, MFA, and account deletion are not
implemented. No public deployment is claimed ready until these choices and the
abuse controls are reviewed.

Expired session rows require periodic maintenance: `DELETE FROM sessions WHERE
expires_at <= now()`. No scheduler is installed yet. This affects storage growth,
not expiry enforcement. Login replacement is not atomic across concurrent requests;
a failed client response can leave an unreachable session until expiry/cleanup.

## References

- https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- https://pkg.go.dev/net/http#CrossOriginProtection
