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
- the data PVC and every other volume become a `tmpfs` with mode 1777 (`--tmpfs-size`, default `4g`), so non-root images can write the way they do with `fsGroup` in the cluster;
- every container gets `--ulimit nofile=1048576:1048576` (`--nofile`, 0 keeps the docker default of 1024, at which aptos' RocksDB dies with "Too many open files").

Keep `--tmpfs-size` at 4g or more. geth and its forks watch free space on the data directory and shut down with "Low disk space. Gracefully shutting down" on a 1g tmpfs. chainsmoke reports that line, and "no space left on device", as WARN "tmpfs too small" rather than FAIL, but the node has still not run its full window.

Chains whose default image is empty (the image has to come from `spec.image`, e.g. aurora) are SKIP "no default image (spec.image required)" unless `--image` supplies one.

### Pods with sidecars

When the pod has native sidecars (init containers with `restartPolicy: Always`, such as the Postgres of the ZK Stack chains) or regular sidecars (UTXO exporters, the solana exporter), chainsmoke runs the whole pod:

1. every pod volume except the config ConfigMap becomes a tmpfs-backed docker volume (`docker volume create --opt type=tmpfs`, capped at `--tmpfs-size`) shared by all containers that mount it;
2. a holder container (`busybox:1.36`, the role of the pause container) mounts all volumes, so the tmpfs stays alive between container starts, and owns the network namespace; every other container runs with `--network container:<holder>`, so `127.0.0.1` is shared as in a pod;
3. `subPath` mounts use `--mount ...,volume-subpath=` (Docker Engine 26+); the holder creates the directories first, because docker refuses a subpath that does not exist. On older engines each subPath gets a volume of its own, which looks the same from inside the container but is not visible through a mount of the whole volume;
4. native sidecars start in spec order; if a sidecar has an exec startup or readiness probe, chainsmoke runs it with `docker exec` every second and waits up to 60s before starting the next container. A sidecar that exits or never gets ready makes the row FAIL with `sidecar <name>: ...`;
5. the main container starts and the regular sidecars start after it. A regular sidecar that exits only adds a note: the verdict is about the main container.

Ordinary init containers (snapshot restore, genesis download) are still not run and show up as a note. Sidecar output goes to `<out>/<chain>.<container>.log`.

After `--duration` (default 90s) the result is:

| Result | When |
|---|---|
| FAIL | the container exited before the window ended, with any exit code, or a log line matches `unknown flag`, `flag provided but not defined`, `unrecognized option`, `invalid argument`, `error parsing`, `failed to parse`, `panic:` (not Rust's `core::panic::` in backtraces), `fatal error:` or `no such file or directory ... config`. For an exit the detail shows that line, else the last `error`/`fatal` line before the exit |
| WARN | still running, but some lines contain the word `error` or `fatal` |
| WARN | the log says the disk is full (`Low disk space`, `no space left on device`), which means `--tmpfs-size` is too small, not that the image is broken |
| PASS | still running, clean log |
| SKIP | the chain has a default image but no sample, or the sample has no image (image-required chain without `--image`) |

```bash
go run ./cmd/chainsmoke --level 1 --chains '^(dash|dogecoin)$' --duration 60s
```

Containers and volumes are removed after the run, also on Ctrl-C. A pulled image is removed once the last job using it finishes, unless you pass `--keep-images` or the image was already on the host when the run first needed it. A full level 1 sweep pulls each of the ~100 node images once, tens of gigabytes in total, so run it in chunks with `--chains`.

Container and volume names are `<name-prefix>-<chain>` plus a suffix for pod parts (`-pod`, `-<sidecar>`, `-<volume>`). Two runs at the same time need different `--name-prefix` values (default `chainsmoke`), otherwise the second removes the containers of the first:

```bash
go run ./cmd/chainsmoke --level 1 --name-prefix smoke2 --chains '^aptos$' --out /tmp/smoke2
```

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

The markdown table goes to stdout and to `<out>/smoke-report.md` (default `./smoke-out`, which is gitignored). Level 1 also writes `<out>/<chain>.log`: the header lines are the exact docker commands that built the pod (volumes, holder and sidecars first, then the main `docker run`), ready to paste in order, and the rest is the main container output. The process exits with 1 if any row is FAIL.

Leftover containers and volumes after a hard kill carry the label `chainsmoke=1`, and `chainsmoke.prefix=<name-prefix>` to clean up one run only:

```bash
docker rm -f $(docker ps -aq --filter label=chainsmoke=1)
docker volume rm $(docker volume ls -q --filter label=chainsmoke=1)
docker rm -f $(docker ps -aq --filter label=chainsmoke.prefix=smoke2)
```

## What it does not catch

A WARN or even a PASS is not proof the node syncs. Chains that need an L1 endpoint, a snapshot or a genesis file from an init container often stop early or complain loudly in this setup; read the log before treating that as a regression. The other direction matters too: an image that ignores the mounted config and quietly runs on its baked-in defaults still passes, so for a new adapter check in the log that the node reports `/config/...` as its config path.
