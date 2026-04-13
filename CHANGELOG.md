# Changelog

All notable changes to this project will be documented in this file.

## Unreleased (2026-04-13)

### Features

- add customHeaders to Helm chart, use /healthz probes, update docs for new features ([b234967](https://github.com/somaz94/static-file-server/commit/b23496748f7d5deda8fc7bbee24c782c937f7206))
- add helmfile examples and include all deploy files in version bump ([73e8cb1](https://github.com/somaz94/static-file-server/commit/73e8cb1b6e112693325711f7ad2d85f648e1161d))
- add /healthz endpoint, file stats in footer, custom response headers ([206bd32](https://github.com/somaz94/static-file-server/commit/206bd32824a26202809dfdeca38c8eb7c597f91d))
- add dark/light mode toggle and show version in footer ([29a8da3](https://github.com/somaz94/static-file-server/commit/29a8da3cf8b247603ca74684d22002ba11d7bb61))

### Builds

- **deps:** bump docker/build-push-action from 6 to 7 (#8) ([#8](https://github.com/somaz94/static-file-server/pull/8)) ([668efd0](https://github.com/somaz94/static-file-server/commit/668efd0e4b25acc622ea98a7c142320ef4769d72))

### Continuous Integration

- add Docker build and push job to release workflow ([a96da54](https://github.com/somaz94/static-file-server/commit/a96da542c998062c4260705182a4b9e03b111d89))

### Chores

- bump version to v0.2.0 ([b128630](https://github.com/somaz94/static-file-server/commit/b128630b5d5a23a607478209904f774221150f63))

### Contributors

- somaz

<br/>

## [v0.1.0](https://github.com/somaz94/static-file-server/releases/tag/v0.1.0) (2026-04-13)

### Features

- add all storage examples, pin deployment.yaml to versioned tag ([b5dc0ff](https://github.com/somaz94/static-file-server/commit/b5dc0ff015685f33a85381cf6f60418e8dc74768))
- add static PV/PVC support, extraVolumes, and NFS example values ([54e1434](https://github.com/somaz94/static-file-server/commit/54e1434d6fefb5f4ad4538a1e9defe3bea107496))
- add version/bump-version, workflow targets, test-helm, and improve ignore files ([eac8630](https://github.com/somaz94/static-file-server/commit/eac86302863bdb5c2b3ed6592d812d14f5624d4b))
- add CI workflows, Helm chart, and documentation ([cf677b7](https://github.com/somaz94/static-file-server/commit/cf677b7aefd886d67119a6a5f92e0f01d5658f9f))
- add Dockerfile, K8s manifests, and deploy/undeploy targets ([ede5f85](https://github.com/somaz94/static-file-server/commit/ede5f85b452cd014fac6254f301d1aa4e3ddc20c))
- add file icons, search, preview modal, tests, and enhanced Makefile ([777c387](https://github.com/somaz94/static-file-server/commit/777c387c465f9a7f339332bc7217084ea725c17f))
- scaffold Go static-file-server with full halverneus feature parity ([8ed27c7](https://github.com/somaz94/static-file-server/commit/8ed27c7fa5dee67cca43ffd185b4755538650952))

### Bug Fixes

- decouple deploy from docker-build to skip unnecessary rebuilds ([4858036](https://github.com/somaz94/static-file-server/commit/4858036b5ae6ed99da60a56463f15a98ef7884d8))

### Code Refactoring

- align workflows with k8s-namespace-sync pattern ([ae2327c](https://github.com/somaz94/static-file-server/commit/ae2327c11ba95bf2ec48acd807fcd55320c13af7))
- extensible PV/PVC with items list, support dynamic and static provisioning ([3c123d0](https://github.com/somaz94/static-file-server/commit/3c123d0952c39e43ad019f6f6df16aaa3a7d1016))

### Documentation

- add test guide, all storage examples, and enhance README with badges ([bf43a5d](https://github.com/somaz94/static-file-server/commit/bf43a5d1c26bd98b2073a948395528d32d154647))
- add version management guide and update README/CLAUDE.md ([b994be5](https://github.com/somaz94/static-file-server/commit/b994be5d5567b65359bb7f02ac29cb1421f2698b))

### Tests

- boost coverage to 93.5% and update Makefile for DockerHub ([2d68279](https://github.com/somaz94/static-file-server/commit/2d682794bbd0faf3a59466b8d718a0e8df1ca7ec))

### Builds

- **deps:** bump dependabot/fetch-metadata from 2 to 3 (#7) ([#7](https://github.com/somaz94/static-file-server/pull/7)) ([6507118](https://github.com/somaz94/static-file-server/commit/650711897012c776a6966225351b52063cf49b13))
- **deps:** bump softprops/action-gh-release from 2 to 3 (#5) ([#5](https://github.com/somaz94/static-file-server/pull/5)) ([251c65f](https://github.com/somaz94/static-file-server/commit/251c65fbb5c9cb22fdcd65bddaa0f3977622177e))
- **deps:** bump docker/setup-buildx-action from 3 to 4 ([18dcf44](https://github.com/somaz94/static-file-server/commit/18dcf447e0de095947d0251ac91e19a7db641ca6))
- **deps:** bump golang from 1.24 to 1.26 in the docker-minor group (#2) ([#2](https://github.com/somaz94/static-file-server/pull/2)) ([558ee43](https://github.com/somaz94/static-file-server/commit/558ee434b593de84d3833856ed013da56874e177))
- **deps:** bump github.com/spf13/cobra in the go-minor group (#1) ([#1](https://github.com/somaz94/static-file-server/pull/1)) ([a12230c](https://github.com/somaz94/static-file-server/commit/a12230c0e8eb798d3f482daa4bf42bf387271a5a))

### Chores

- add helmfile test deployment to gitignore ([a15ea41](https://github.com/somaz94/static-file-server/commit/a15ea410935b62bab1ce6779753b08d2e6aa45b3))

### Contributors

- somaz

<br/>

