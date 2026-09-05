# Third-party notices

The root MIT license covers original Ad Agent code and documentation. It does not
relicense dependencies, brands, provider services, or bundled demonstration media.

- Sandbox photos/videos retain the Pexels license and creator credits in
  [media attribution](web/public/sandbox/creatives/ATTRIBUTION.md). Keep that file with
  redistributed media. The fictional advertiser does not imply creator endorsement.
- Pi packages are separately MIT-licensed. OpenAI Codex is separately Apache-2.0-licensed.
  Preserve their upstream license/notice files when distributing them.
- Built-in Runtime uses the separately Apache-2.0-licensed
  `github.com/z-chenhao/J/J-agent` loop dependency. Its product name does not change that
  dependency's identity or license; preserve upstream notices when distributing it.
- React, Tailwind CSS and Radix packages retain their own licenses. Lucide icons retain
  their ISC license. Other transitive dependencies are identified by the lockfiles.
- Claude Agent SDK declares `SEE LICENSE IN README.md`; its installed package documents
  Anthropic terms and data policies. It is a separately installed integration, not
  software relicensed under this repository's MIT license.
- TikTok Marketing API access and model/OAuth services require operator authorization
  and remain subject to their providers' terms. No credentials or service entitlements
  are supplied with this source release.

Source distributions exclude `node_modules`, compiled bridges, credentials, state
databases and provider transcripts. Binary distributions need a dependency-license audit
and complete applicable notices; this alpha supplies source, not a bundled SDK binary.
