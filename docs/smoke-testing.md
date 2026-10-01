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

### Identity checks

Staying up for 90 seconds proves little on its own. An OP Stack image that never reads the mounted config starts as Ethereum mainnet, keeps its chain under `/root/.ethereum` inside the container and runs happily for the whole window; 14 chains passed that way before these checks existed. So when the window ends and the main container is still running, chainsmoke looks at what the node did:

1. **Data on the volume.** A busybox helper joins the PID namespace of the main container (`--pid container:<main> --cap-add SYS_PTRACE`, the capability is for nodes that do not run as root) and runs `du -sk /proc/1/root/data`. That sees the tmpfs of a single container and the shared volume of a pod alike, and needs nothing from the node image. Under 64 KiB is FAIL `node wrote nothing to /data (N KiB): config/datadir not applied`: every client that takes its datadir from the operator creates its database there right at start. A chain whose node legitimately writes nothing in the first minutes goes into `dataCheckExempt` in `cmd/chainsmoke/identity.go` with the reason, which then shows as a note. The list is empty; an exemption is not a way to silence a datadir that is not applied.
2. **EVM chain id.** For chains listed in `internal/adapters/chain_identity.go`, a helper in the pod network namespace (`--network container:<main or holder>`) POSTs `eth_chainId` to `127.0.0.1` on the main container port named `evm-rpc`, else `rpc`, else `http` (avalanche adds `/ext/bc/C/rpc`). A different id is FAIL `chain id X, expected Y`. No answer after three tries two seconds apart turns a PASS into WARN `rpc not ready: ...`: plenty of nodes wait for L1 or a peer before they open RPC, and that is not an identity problem. A node that ignores its config usually has no HTTP RPC either (geth-style clients keep it off by default), so for that case the data check is the one that fires.
3. **CometBFT network.** For the Cosmos chains in the same file, the helper GETs `/status` on port 26657 and compares `node_info.network` with the expected chain-id: FAIL `network X, expected Y` on a mismatch, WARN `cometbft rpc not ready` without an answer.

The expected values cover mainnet, plus testnet for the adapters that pin `network: testnet` to one network (ethereum: sepolia, bsc: chapel, avalanche: fuji); elsewhere "testnet" is not tied to a network and there is nothing to expect. They are taken from chainlist.org for EVM ids and cosmos/chain-registry for chain-ids. A PASS row lists what was verified, e.g. `running after 1m30s; /data 51 MiB; chain id 1`. EVM chains without an expected id: sei and kava (the EVM JSON-RPC port is not exposed by the main container), berachain (the main container is the consensus client), hyperliquid (no EVM JSON-RPC in the node).

### Result

After `--duration` (default 90s) the result is:

| Result | When |
|---|---|
| FAIL | the container exited before the window ended, with any exit code, or a log line matches `unknown flag`, `flag provided but not defined`, `unrecognized option`, `invalid argument`, `error parsing`, `failed to parse`, `panic:` (not Rust's `core::panic::` in backtraces), `fatal error:` or `no such file or directory ... config`. For an exit the detail shows that line, else the last `error`/`fatal` line before the exit |
| FAIL | still running, but an identity check failed: nothing on the data volume, wrong `eth_chainId` or wrong CometBFT network. The identity failure comes first in the detail |
| WARN | still running, but some lines contain the word `error` or `fatal` |
| WARN | the log says the disk is full (`Low disk space`, `no space left on device`), which means `--tmpfs-size` is too small, not that the image is broken |
| WARN | clean log, but the RPC needed for the chain id check did not answer |
| PASS | still running, clean log, identity checks passed or not applicable |
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

A WARN or even a PASS is not proof the node syncs. Chains that need an L1 endpoint, a snapshot or a genesis file from an init container often stop early or complain loudly in this setup; read the log before treating that as a regression. The other direction matters too: the identity checks catch an image that ignores the mounted config when that changes its datadir or its chain, but not a config setting that leaves both alone (ports, pruning, peers), and not a chain without an expected id. For a new adapter, still check in the log that the node reports `/config/...` as its config path.
