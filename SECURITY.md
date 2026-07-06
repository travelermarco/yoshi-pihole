# Security

Yoshi Pi-hole is a single-user, single-machine tool. Its security model is intentionally simple:

- The REST API and dashboard have **no authentication** and bind only to `127.0.0.1` — they are never reachable from the network, only from processes running on the same Mac. This is a deliberate design choice (see the README), not an oversight.
- The DNS daemon runs as **root** only because binding port 53 requires it; it does not otherwise need or use elevated privileges.
- `scripts/install.sh` and `scripts/uninstall.sh` always ask for explicit confirmation before touching system files or DNS settings, and back up prior DNS configuration before changing it.

## Reporting a vulnerability

If you find an actual security issue (e.g. something that could be exploited from *outside* localhost, a privilege escalation in the install/uninstall scripts, or a flaw unrelated to the accepted no-auth-on-localhost model above), please open a GitHub issue or contact the maintainer directly rather than disclosing it publicly first.
