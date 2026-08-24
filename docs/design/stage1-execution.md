# Stage 1 execution — mlx-c 0.32 canonical cut (branch `stage1/mlx-v0.32.0`)

Date: 2026-08-23 · Executed by session F9AA979C · Status: complete locally, unpushed
Supersedes: the rehearsal in `~/tmp/mlxc-dryrun-20260823` (kept for reference)
Governing doc: the ratified re-derived path (`~/tmp/agent-collab/ml-explore/20260823-mlxc-rederived-path.md`)
Tooling branch: `gates/tooling-fixes` (three commits on top of `gates/compat-monotonicity`)

## Verification result

`mlx-c-gen check` against baseline `d0bfeb5:codegen/mlxc-capi.lock.json` and upstream
headers `upstream/mlx-v0.32.0`, MLX source pinned at exactly `v0.32.0`:

```
upstream parity: 632 shared, 19 ours-only, 4 divergent (4 waived)   PASS
lock monotonicity: 4 added, 0 removed, 3 waived removals            PASS
generated files: 38 equal, 0 different                              PASS
```

The final surface is a strict superset of the consumer-pinned d0bfeb5 lock modulo
three declared renames; `mlx_astype_copy` is preserved.

## Commit walk (atomic, in order)

| commit | content |
|---|---|
| `037045b` | Merge `upstream/mlx-v0.32.0`; CMakeLists keeps our MLX_API event-export patch; generated conflicts resolve toward regeneration |
| `2a2ecbd` | Manifest: not_c_api→excluded_by_policy migration (34 entries), parity flips for count_nonzero/flip/unstack base names, math_mode hook_api carry-in |
| `666e93b` | Inventory classifications for event.{h,cpp}, private/event.h, stream_debug_trace.* |
| `8c2280e` | jaccl custom spec + implementation: mlx_jaccl_config_new_out ctor from gen lineage |
| `8c14316` | removals.yaml: three declared renames with resolutions |
| `ff16b3e` | Regenerated mlx/c/* + API lock + lock TU |
| `b925db8` | compat-waivers.yaml: four declared shared-symbol divergences |

## Tooling fixes that made this work (branch `gates/tooling-fixes`)

1. `mlxcgen/types: register std::optional<bool>` — the real root cause of the
   missing astype_copy binding: optional-primitive registration listed float,
   int, Dtype but not bool, so upstream's `astype(a, dtype, copy, s)` overload
   was silently dropped. (Dry-run hypothesis "inline wrapper shadowing" was
   wrong; the parser captures both overloads fine.)
2. `mlxcgen/hooks: keep node-namer get_name result alive` — lifetime fix in the
   generator template (thread_local keep-string), so the generated file no
   longer needs a hand patch.
3. `tools/lockgen` — emits codegen/mlxc-capi.lock.json + lock.c; until an emit
   subcommand exists, regeneration flows need this.

## What remains for Stage-1 green (owner: Travis / B4F7C2FF)

1. Review + push `gates/tooling-fixes` and `stage1/mlx-v0.32.0`; ff local
   `mlx-0.32` naming per plan (pushes are Travis's).
2. Record resulting SHA as `<NEW032>`.
3. Build the dylib (cmake + MLX v0.32.0 fetch) — not exercised here; the dry run
   and this cut verify generation and gates, not compilation.
4. Phase 2: publish libs release, repoint mlx-go, regen Go bindings.
5. Decision points still open (gate report): fast_metal_kernel_new math_mode
   naming, metallib_path constness, cumsum/cumprod dtype under base name,
   require_upstream_parity default-on.

## D10 evidence (generator home)

Running the gen/main-lineage tooling against the branch-lineage tree worked
cleanly with inputs resolved per-tree (manifest/custom/inventory/removals all
read from -root). That supports either D10 outcome; the pseudo-version pinning
probe for branch-in-fork remains open.
