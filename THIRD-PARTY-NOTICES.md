# Third-party notices

Yoshi Pi-hole is MIT-licensed (see [LICENSE](LICENSE)). It statically links the following direct dependencies, all under permissive licenses compatible with MIT distribution:

| Dependency | License |
|---|---|
| [github.com/miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause |
| [github.com/getlantern/systray](https://github.com/getlantern/systray) | Apache-2.0 |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | MIT / Apache-2.0 (dual) |

Each of these pulls in further transitive dependencies (see `go.sum`); all are standard permissive Go-ecosystem licenses (BSD/MIT/Apache family). Run `go list -m all` for the full dependency graph, or a tool like [go-licenses](https://github.com/google/go-licenses) for an automated per-package license report.
