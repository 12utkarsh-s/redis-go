"""Poll the redis-go server's INFO keyspace counter and log it over time."""
import re
import socket
import sys
import time

DURATION = float(sys.argv[1]) if len(sys.argv) > 1 else 60.0
INTERVAL = 0.5

INFO = b"*1\r\n$4\r\nINFO\r\n"
PAT = re.compile(rb"db0:keys=(\d+)")


def main():
    s = socket.create_connection(("localhost", 7379), timeout=5)
    start = time.time()
    rows = []
    while time.time() - start < DURATION:
        s.sendall(INFO)
        data = s.recv(65536)
        m = PAT.search(data)
        if not m:
            print("unparsed: %r" % data[:120], flush=True)
            break
        keys = int(m.group(1))
        t = time.time() - start
        rows.append((t, keys))
        print("%6.2f  %d" % (t, keys), flush=True)
        time.sleep(INTERVAL)
    s.close()

    if rows:
        vals = [k for _, k in rows]
        print("\n--- summary ---")
        print("samples : %d" % len(vals))
        print("min     : %d" % min(vals))
        print("max     : %d" % max(vals))
        print("final   : %d" % vals[-1])
        drops = [(rows[i - 1][1], vals[i]) for i in range(1, len(vals))
                 if vals[i] < rows[i - 1][1]]
        print("drops   : %d %s" % (len(drops), drops[:20]))


if __name__ == "__main__":
    main()
