# Release Process

## Versioning

The project follows [Semantic Versioning](https://semver.org/). The API group is currently `v1alpha1`, meaning breaking changes to the CRD schema may occur in minor releases until the API is promoted to `v1beta1`.

## Pre-Release Checklist

Before tagging a release, verify the following:

- [ ] `charts/chainplane/Chart.yaml` — bump both `version` (chart semver) and `appVersion` (operator image tag, e.g. `v0.X.Y`)
- [ ] `CHANGELOG.md` — add an entry for the new version describing notable changes, new chains, bug fixes
- [ ] All CI workflows are green on the commit you intend to tag (check Actions tab)
- [ ] No pending lint or vet issues (`go vet ./...` passes locally)

## How to Release

Tagging a commit triggers the `release.yml` workflow automatically:

```bash
git tag v0.X.Y
git push origin v0.X.Y
```

That's it. The workflow handles everything else.

## What release.yml Does

1. **Sanity check** — runs `go build ./... && go vet ./...` to catch compilation errors.
2. **Multi-arch image build** — builds `linux/amd64` and `linux/arm64` images via Docker Buildx and pushes them to GHCR:
   - `ghcr.io/tazhate/chainplane:v0.X.Y`
   - `ghcr.io/tazhate/chainplane:latest`
3. **Helm chart package** — runs `helm package charts/chainplane --version 0.X.Y` (the `v` prefix is stripped automatically), producing `chainplane-0.X.Y.tgz`.
4. **GitHub Release** — creates a GitHub Release on the tag with auto-generated release notes and attaches the Helm chart `.tgz` as a release asset.

## Post-Release Steps

### Verify the image is publicly pullable

GHCR packages are **private by default** on the first push to a new repository. After the first release:

1. Go to `https://github.com/tazhate?tab=packages`
2. Find `chainplane`
3. Click **Package settings** → **Change visibility** → **Public**

Subsequent pushes to the same package will remain public. Verify with:

```bash
docker pull ghcr.io/tazhate/chainplane:v0.X.Y
```

### Verify the Helm chart asset

Check that the `.tgz` is attached to the GitHub Release:

```bash
gh release view v0.X.Y --repo tazhate/chainplane
```

### Update the Helm repo index (if applicable)

If you maintain a static Helm repository (e.g. via GitHub Pages), re-run `helm repo index` and push the updated `index.yaml`.
