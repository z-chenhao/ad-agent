# Alpha roadmap

These are priorities for the existing `0.1.0-alpha.1`, not promised release dates,
implemented capabilities, or a funding commitment. The primary maintainer accepts scope
through issues and reviews. Provider availability and platform approval are external gates.

| Priority               | Useful outcome                                                                                   | Done when                                                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Contributor onboarding | A new contributor can reproduce a report and harness run without secrets                         | Fresh source install passes `make smoke`; installation issues have reproducible OS/tool details                        |
| Runtime reliability    | Provider/protocol changes cannot silently break application authority or conversation continuity | Targeted transport, tool pairing, cancellation and checkpoint regressions pass for each changed adapter                |
| Evidence quality       | Findings distinguish observed data, hypotheses, incomplete attribution and profitability         | Reported failures become deterministic structural tests where possible; semantic quality remains separately reviewed   |
| Security maintenance   | External reports reach a private channel and receive reproducible fixes                          | Private reporting is enabled by the repository owner; fixes include regression evidence without disclosing secrets     |
| TikTok acceptance      | A permitted account validates actual read and write semantics                                    | Developer approval, scoped authorization, controlled reads and separately approved write/read-back checks are recorded |

## Where contributions help now

- Reproduce source installation on a clean Linux or macOS environment; Windows support
  needs separate evidence rather than a compatibility claim.
- Add minimal regressions for public commentary/tool ordering, cancellation and history
  restoration after a runtime change.
- Review nullable metrics, attribution windows and approval previews using fictional data.
- Reproduce documented Sandbox invariants across seeds without interpreting results as
  predictions of a platform's proprietary delivery system.

See [contributing](../CONTRIBUTING.md) and [validation](validation.md). The existing
[capability map](capabilities.md) remains the source of truth for implemented operations.

## Not planned for this alpha

No multi-tenant hosting, autonomous live-ad approvals, universal plugin SDK, calibrated
platform simulator, or new provider merely to expand a feature list. Meta integration is
future work. Automatic paid PR agents and broad live-model sweeps are not enabled.

We will judge progress by reproducible fixes, reviewed contributions and documented user
needs. Stars, synthetic advertising volume and test counts are not adoption or business-impact evidence.
