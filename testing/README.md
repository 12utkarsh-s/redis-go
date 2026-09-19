# Manual eviction tests

Two stdlib-only Python scripts for exercising the eviction strategies against a
running server. Neither is a unit test - start the server first:

```sh
go run .
```

## lru_semantics.py

Checks that `allkeys-lru` evicts the *least recently used* keys rather than
arbitrary ones.

```sh
python3 testing/lru_semantics.py
```

Seeds 60 keys, sleeps 3s so the 1-second clock in `getCurrentClock` can
advance, re-reads the first 20 to make them hot, then forces one eviction cycle
via the `LRU` command and reports who survived.

Expect roughly **18-20/20 hot surviving** and **0-2/40 cold surviving**.
Sampled LRU is approximate, so a couple either way is normal - but survival
near 33% on both rows means eviction has degenerated to random, which is what
a bypassed eviction pool looks like.

Assumes the defaults in `config`: `KeysLimit = 100` and `EvictionRatio = 0.40`,
giving an evictCount of 40. Adjust `HOT`/`COLD` if you change those.

## monitor.py

Polls `INFO` and prints the `db0:keys` count over time.

```sh
go run ./storm/set &        # write load
python3 testing/monitor.py 45
```

The count should sawtooth and plateau at `KeysLimit`, never above it - that
shows eviction keeping pace with writes. A count that climbs past the limit
means a cycle is freeing less than `evictCount`.

Note the floor looks higher than `KeysLimit - evictCount` because the poll
interval (500ms) is wider than the gap between writes, so the true trough has
already refilled by the next sample.
