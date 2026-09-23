"""Does allkeys-lfu actually evict the LEAST frequently used keys?

Seed 60 keys - every key leaves its SET at counter 6 - then hammer the first 20
with repeated GETs so their logarithmic counters climb above the rest. Force
one eviction cycle (evictCount = 0.40 * 100 = 40) and see who survived. A
correct LFU keeps the 20 hot keys and evicts the 40 cold ones, regardless of
how recently the cold ones were written.

Start the server with LFU selected:

    go run . -eviction-strategy allkeys-lfu
"""
import re
import socket

HOT, COLD = 20, 40
TOTAL = HOT + COLD

# The increment is probabilistic: at counter 6 it fires with p = 1/11, and each
# further step costs ~10 more accesses. This is enough to lift the hot set a
# couple of counts clear of the cold set, which is all the ranking needs.
HITS = 60


def cmd(*parts):
    out = ("*%d\r\n" % len(parts)).encode()
    for p in parts:
        b = str(p).encode()
        out += b"$%d\r\n%s\r\n" % (len(b), b)
    return out


class Client:
    def __init__(self):
        self.s = socket.create_connection(("localhost", 7379), timeout=5)

    def call(self, *parts):
        self.s.sendall(cmd(*parts))
        return self.s.recv(65536)

    def keys(self):
        return int(re.search(rb"db0:keys=(\d+)", self.call("INFO")).group(1))


c = Client()
print("start keys       :", c.keys())

for i in range(TOTAL):
    c.call("SET", "key%d" % i, i)
print("after %d SETs    : %d" % (TOTAL, c.keys()))

# No sleep here on purpose: unlike LRU, age is not what LFU ranks on. The cold
# keys stay the most recently written ones, so a pass that survives this is
# ranking on frequency and not on the clock.
for _ in range(HITS):
    for i in range(HOT):
        c.call("GET", "key%d" % i)
print("hit key0..key%d %d times each (now hot)" % (HOT - 1, HITS))

before = c.keys()
c.call("LFU")                      # force one evictAllkeysLFU() cycle
after = c.keys()
print("keys %d -> %d (freed %d)" % (before, after, before - after))

hot_alive = sum(1 for i in range(HOT)
                if c.call("GET", "key%d" % i) != b"$-1\r\n")
cold_alive = sum(1 for i in range(HOT, TOTAL)
                 if c.call("GET", "key%d" % i) != b"$-1\r\n")

print("\n--- result ---")
print("hot  surviving : %d / %d" % (hot_alive, HOT))
print("cold surviving : %d / %d" % (cold_alive, COLD))
print("verdict        : %s" % ("LFU respected"
                               if hot_alive > cold_alive else "NOT LFU-like"))
