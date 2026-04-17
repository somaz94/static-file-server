# Changelog

All notable changes to this project will be documented in this file.

## [v0.5.0](https://github.com/somaz94/static-file-server/compare/v0.4.1...v0.5.0) (2026-04-17)

### Features

- **helm:** add Gateway API HTTPRoute template with HTTPS redirect support ([1e94edb](https://github.com/somaz94/static-file-server/commit/1e94edb8edbc25bc17e7a29779fae28d1b350b63))
- add security headers middleware, gzip/buffer pooling, and CLI tests ([34dac42](https://github.com/somaz94/static-file-server/commit/34dac429b783e43a1a9a62cfc777bf619ed7bf11))

### Documentation

- add deploy directory README and link from main docs ([1c9d7c8](https://github.com/somaz94/static-file-server/commit/1c9d7c8c3f43c2a07b126351f0dc5fe510f3d552))
- add directory listing UI screenshot to README ([6f1e6bc](https://github.com/somaz94/static-file-server/commit/6f1e6bc1928519b9d5aed54c2d475ab79671f7f6))
- slim down README and extract UI docs ([a67e7cc](https://github.com/somaz94/static-file-server/commit/a67e7cc177734abea8c8bd7f8d5cad6beef6b7be))

### Chores

- bump version to v0.5.0 ([098c8c4](https://github.com/somaz94/static-file-server/commit/098c8c4d502c7c0515d9ba9a32d9f4b0e2433620))

### Contributors

- somaz

<br/>

## [v0.4.1](https://github.com/somaz94/static-file-server/compare/v0.4.0...v0.4.1) (2026-04-13)

### Features

- add missing compression extensions and direct download for single file ([11cb370](https://github.com/somaz94/static-file-server/commit/11cb370b66ce8a3ff0a1a513de7b552ce3075702))

### Bug Fixes

- avoid SIGPIPE in smoke test body_contains with here-string ([03c4e42](https://github.com/somaz94/static-file-server/commit/03c4e425bfda85f4cf5f739eb330a0a6e2cfa8ea))
- copy full URL instead of path-only in directory listing ([a417541](https://github.com/somaz94/static-file-server/commit/a417541d8cca3cc0e23e0c4fc495c10647fc92d9))
- enable metrics in local deploy for smoke tests ([92e9986](https://github.com/somaz94/static-file-server/commit/92e9986402ccbb79d445ca88db6cacf0d3fc84b6))
- resolve gofmt, ineffassign, and gocyclo warnings ([ce11933](https://github.com/somaz94/static-file-server/commit/ce119330e572e389afda87d0b5a2ce2baa64c916))
- correct Prometheus histogram format and metrics key parsing ([e2a5af3](https://github.com/somaz94/static-file-server/commit/e2a5af34fce73aba7ddc01d386723c3bc6d4bd28))

### Code Refactoring

- unify ResponseWriter wrappers and add http.Flusher support ([585ec21](https://github.com/somaz94/static-file-server/commit/585ec216bb8e0efed63ed79de513b0cff1596484))
- optimize compression extension check from O(n) to O(1) ([5ad4244](https://github.com/somaz94/static-file-server/commit/5ad4244769e74d9d13f8d6b3170e091f65c37cc8))

### Documentation

- update Prometheus metrics documentation and add metrics smoke test ([4b2696f](https://github.com/somaz94/static-file-server/commit/4b2696f5d523eef6063e7a60eb4b3336f26271d8))

### Tests

- add Flush tests and remove unused totalRequests ([6e8f583](https://github.com/somaz94/static-file-server/commit/6e8f583ffd6d5894aa5e5fea6b4289eff9418cd8))

### Chores

- bump version to v0.4.1 ([bd9b9a5](https://github.com/somaz94/static-file-server/commit/bd9b9a5a08c4983897cb6a201c792be7e90d9f9d))

### Contributors

- somaz

<br/>

## [v0.4.0](https://github.com/somaz94/static-file-server/compare/v0.3.0...v0.4.0) (2026-04-13)

### Features

- add advanced directory listing UI with grid view, batch download, and gallery preview ([666a4ee](https://github.com/somaz94/static-file-server/commit/666a4eeecdeb8dfaa30588f2e6ce923b534ab515))

### Bug Fixes

- Makefile ([7de4a36](https://github.com/somaz94/static-file-server/commit/7de4a36db96cb4c1eeee3c0e7de0f3bb7747beb6))
- harden batch download, fix JS bugs, update docs and deploy targets ([6c4242a](https://github.com/somaz94/static-file-server/commit/6c4242a59a36cb36b19c6f94a54b5611891b639e))

### Code Refactoring

- switch deploy to local binary, add deploy-docker, rename deploy targets ([9d0428f](https://github.com/somaz94/static-file-server/commit/9d0428f5ea56cce2a1c76047882a7cfcf81c7b35))

### Contributors

- somaz

<br/>

## [v0.3.0](https://github.com/somaz94/static-file-server/compare/v0.2.0...v0.3.0) (2026-04-13)

### Features

- add gitlab-mirror workflow ([99b7c7f](https://github.com/somaz94/static-file-server/commit/99b7c7f6daef75a6bd4d4a787b8fdca258222b6a))
- add SPA mode, gzip compression, metrics, JSON logging, dot file filtering ([dfadb89](https://github.com/somaz94/static-file-server/commit/dfadb8954c8fe9eeeb22e25a3ab067dc8c75e345))

### Bug Fixes

- remove halverneus reference, fix MD5 comments to SHA-256, remove TLS10 support ([8d2ef5b](https://github.com/somaz94/static-file-server/commit/8d2ef5b831798db76121431cdfa27de97b9fe8ae))
- **ci:** correct workflow_run trigger name in changelog-generator ([07c7aae](https://github.com/somaz94/static-file-server/commit/07c7aae7a9c96c0f3fcf2fd443931e09736640c3))

### Code Refactoring

- improve security, reliability, and observability ([603e8e7](https://github.com/somaz94/static-file-server/commit/603e8e72689452f5965ea887906778d37a5f121d))

### Documentation

- add SPA, compression, metrics, JSON logging, dot file config to helm/yaml/docs ([4d6739b](https://github.com/somaz94/static-file-server/commit/4d6739b5236564f6b153c842f22c5398b4feafb3))
- add deployment smoke tests guide ([433a9b6](https://github.com/somaz94/static-file-server/commit/433a9b66c163285c45635584ceeb57a1bdc84e1a))

### Tests

- add graceful shutdown tests to reach 90%+ coverage ([807d73b](https://github.com/somaz94/static-file-server/commit/807d73b7b7e03661180e9c222cea6ce7d9f14fc9))

### Chores

- bump version to v0.3.0 ([ba4eea5](https://github.com/somaz94/static-file-server/commit/ba4eea51613160de5c7968dd5419d9043f0be55a))

### Contributors

- somaz

<br/>

## [v0.2.0](https://github.com/somaz94/static-file-server/compare/v0.1.0...v0.2.0) (2026-04-13)

### Features

- add customHeaders to Helm chart, use /healthz probes, update docs for new features ([b234967](https://github.com/somaz94/static-file-server/commit/b23496748f7d5deda8fc7bbee24c782c937f7206))
- add helmfile examples and include all deploy files in version bump ([73e8cb1](https://github.com/somaz94/static-file-server/commit/73e8cb1b6e112693325711f7ad2d85f648e1161d))
- add /healthz endpoint, file stats in footer, custom response headers ([206bd32](https://github.com/somaz94/static-file-server/commit/206bd32824a26202809dfdeca38c8eb7c597f91d))
- add dark/light mode toggle and show version in footer ([29a8da3](https://github.com/somaz94/static-file-server/commit/29a8da3cf8b247603ca74684d22002ba11d7bb61))

### Builds

- **deps:** bump actions/github-script from 8 to 9 (#9) ([#9](https://github.com/somaz94/static-file-server/pull/9)) ([c85ad68](https://github.com/somaz94/static-file-server/commit/c85ad681b278c09a5bac155922d4fd86149408c6))
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

