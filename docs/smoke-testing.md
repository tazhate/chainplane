# Smoke testing node images

`cmd/chainsmoke` checks chain images on a workstation with docker. It never syncs a chain: level 0 talks to registries only, level 1 runs each node for about a minute and reads its logs. Use it after `versioncheck --update` or before a major version bump, when the question is "does the new image still accept the flags and config the operator renders?"

## Level 0: registry check

For every chain and client in `internal/adapters/versions_gen.go`, chainsmoke runs `docker buildx imagetools inspect` and checks that the tag exists and that a `linux/amd64` image is in it. Each distinct reference is inspected once, so the eight chains sharing `op-geth:v1.101702.1` cost one request. A 429 from the registry gets one retry after 60 seconds.

```bash
go run ./cmd/chainsmoke --level 0
go run ./cmd/chainsmoke --level 0 --chains '^(bitcoin|ethereum|dash)$'
```

A full sweep takes a few minutes at the default `--parallel 6`.

## Level 1: run the node

For each `config/samples/chains_v1alpha2_chaininstance_*.yaml`, chainsmoke renders the pod template with the same code the reconciler uses (`controller.RenderPodTemplate`) and turns the main `node` container into `docker run --platform linux/amd64`:

- `command` becomes `--entrypoint` plus leading args, `args` follow the image name, the same split Kubernetes makes;
- `$(VAR)` references in env, command and args are expanded the way the kubelet expands them; Secret and ConfigMap env refs get the value `smoke`;
- the rendered config file (`adapter.ConfigTemplate`) is bind-mounted read-only at the config mount path;
- the data PVC and every other volume become a `tmpfs` with mode 1777 (`--tmpfs-size`, default `4g`), so non-root images can write the way they do with `fsGroup` in the cluster.

Init containers (snapshot restore, genesis download) and sidecars are not run. They show up as notes in the report.

After `--duration` (default 90s) the result is:

| Result | When |
|---|---|
| FAIL | the container exited before the window ended, with any exit code, or a log line matches `unknown flag`, `flag provided but not defined`, `unrecognized option`, `invalid argument`, `error parsing`, `failed to parse`, `panic:`, `fatal error:` or `no such file or directory ... config` |
| WARN | still running, but some lines contain the word `error` or `fatal` |
| PASS | still running, clean log |
| SKIP | the chain has a default image but no sample |

```bash
go run ./cmd/chainsmoke --level 1 --chains '^(dash|dogecoin)$' --duration 60s
```

The container is removed after the run, and so is the image unless you pass `--keep-images`. A full level 1 sweep pulls each of the ~100 node images once, tens of gigabytes in total, so run it in chunks with `--chains`.

## Testing a bump before it lands

`--image chain[/client]=ref` replaces the image without editing code. Without a client it applies to every client of the chain. The flag can be repeated:

```bash
go run ./cmd/chainsmoke --level 1 --chains '^ethereum$' \
  --image ethereum/geth=ethereum/client-go:v1.18.0
```

`--changed-since <git-ref>` limits the run to chains whose entry in `versions_gen.go` differs from that ref, which is the usual check after a `versioncheck` pass:

```bash
go run ./cmd/chainsmoke --level 0 --changed-since origin/master
go run ./cmd/chainsmoke --level 1 --changed-since origin/master
```

## Output

The markdown table goes to stdout and to `<out>/smoke-report.md` (default `./smoke-out`, which is gitignored). Level 1 also writes `<out>/<chain>.log`: the first line is the exact `docker run` command, ready to paste, and the rest is the container output. The process exits with 1 if any row is FAIL.

Leftover containers after a hard kill carry the label `chainsmoke=1`:

```bash
docker rm -f $(docker ps -aq --filter label=chainsmoke=1)
```

## What it does not catch

A WARN or even a PASS is not proof the node syncs. Chains that need an L1 endpoint, a snapshot or a genesis file from an init container often stop early or complain loudly in this setup; read the log before treating that as a regression. The other direction matters too: an image that ignores the mounted config and quietly runs on its baked-in defaults still passes, so for a new adapter check in the log that the node reports `/config/...` as its config path.
