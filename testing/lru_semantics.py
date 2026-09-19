"""Does allkeys-lru actually evict the LEAST recently used keys?

Seed 60 keys, wait so their clock ages, then re-read the first 20 (making them
hot). Force one eviction cycle (evictCount = 0.40 * 100 = 40) and see who
survived. A correct LRU keeps the 20 hot keys and evicts the 40 cold ones.
"""
import re
import socket
import time

HOT, COLD = 20, 40
TOTAL = HOT + COLD


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

# Let the 1-second clock advance so hot/cold are distinguishable.
time.sleep(3)

for i in range(HOT):
    c.call("GET", "key%d" % i)
print("touched key0..key%d (now hot)" % (HOT - 1))

before = c.keys()
c.call("LRU")                      # force one evictAllkeysLRU() cycle
after = c.keys()
print("keys %d -> %d (freed %d)" % (before, after, before - after))

hot_alive = sum(1 for i in range(HOT)
                if c.call("GET", "key%d" % i) != b"$-1\r\n")
cold_alive = sum(1 for i in range(HOT, TOTAL)
                 if c.call("GET", "key%d" % i) != b"$-1\r\n")

print("\n--- result ---")
print("hot  surviving : %d / %d" % (hot_alive, HOT))
print("cold surviving : %d / %d" % (cold_alive, COLD))
print("verdict        : %s" % ("LRU respected"
                               if hot_alive > cold_alive else "NOT LRU-like"))
