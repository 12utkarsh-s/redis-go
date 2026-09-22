package server

import (
	"log"
	"net"
	"os"
	"redis-go/config"
	"redis-go/core"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

var expiryCronLastExecution = time.Now()

// Engine states are bit flags rather than an enum, because the engine can be
// draining a batch and shutting down at the same time. WaitForSignal OR-s in
// SHUTTING_DOWN the moment a signal arrives, which leaves BUSY untouched: the
// engine can no longer claim new work, but an in-flight batch stays visible so
// the handler can wait for it to finish before rewriting the AOF.
//
// Only three values are ever observed: WAITING, BUSY, and BUSY|SHUTTING_DOWN.
// The transitions are:
//
//	WAITING           --CAS(engine)--> BUSY               claim a batch
//	BUSY              --CAS(engine)--> WAITING            batch done
//	WAITING or BUSY   --Or(handler)--> |SHUTTING_DOWN     signal received
//	BUSY|SHUTTING_DOWN --Store(engine)--> SHUTTING_DOWN   drained, engine exits
//
// ADDING A FOURTH STATE WILL SILENTLY BREAK THIS. Both engine transitions are
// CompareAndSwap calls that compare the *whole word*, not individual bits:
// CompareAndSwap(BUSY, WAITING) succeeds only when the status is exactly BUSY.
// That is what makes it fail once SHUTTING_DOWN is OR-ed in, which is how the
// engine learns to stop. If another flag can be OR-ed in concurrently, those
// CAS calls start failing for the wrong reason and the engine shuts itself
// down spuriously. A new flag means rewriting both transitions as CAS loops
// that read the current value and preserve the bits they do not own.

const EngineStatus__WAITING int32 = 1 << 1
const EngineStatus__BUSY int32 = 1 << 2
const EngineStatus__SHUTTING_DOWN int32 = 1 << 3

var eStatus atomic.Int32
var connectedClients map[int]*core.Client

func init() {
	eStatus.Store(EngineStatus__WAITING)
	connectedClients = make(map[int]*core.Client)
}

func RunAsyncTCPServer(wg *sync.WaitGroup) error {
	defer wg.Done()
	defer func() {
		eStatus.Store(EngineStatus__SHUTTING_DOWN)
	}()

	log.Println("Starting async tcp server on ", config.Host, config.Port)

	// maxClients does double duty: the listen backlog (how many completed
	// connections the kernel will hold for us before refusing new ones) and the
	// size of the buffer kqueue writes ready events into. Both are ceilings on
	// how much work can pile up between two iterations of the event loop.
	maxClients := 20000

	// Scratch space kqueue fills in on each wakeup. Allocated once and reused
	// for every iteration so the hot loop does not allocate.
	events := make([]unix.Kevent_t, maxClients)

	//
	// This is the LISTENING socket, and it is the only one created by hand.
	//   AF_INET     - IPv4 addressing (an AF_INET6 socket would speak IPv6)
	//   SOCK_STREAM - a reliable, ordered byte stream, i.e. TCP semantics
	//   0           - let the kernel pick the default protocol for that pair,
	//                 which is TCP
	//
	// Right now it is an anonymous endpoint: it has a type but no address, and
	// it cannot receive anything. Bind and Listen below are what turn it into a
	// server.
	serverFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal("Error while creating Socket ", err)
		return err
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Println("Error while closing Socket ", err)
		}
	}(serverFD)

	// Non-blocking means Accept returns EAGAIN instead of parking the goroutine
	// when no connection is pending. The whole design depends on this: one
	// thread drives every client, so a single blocking call would stall all of
	// them. We only ever call Accept after kqueue has told us it will succeed,
	// but this guarantees a spurious wakeup cannot freeze the loop.
	if err = unix.SetNonblock(serverFD, true); err != nil {
		return err
	}

	// Bind attaches a local address to the socket: "this IP, this port is
	// mine". The kernel now routes inbound TCP packets for that address here.
	// Without it the socket has no address for clients to reach.
	//
	// This is also where "address already in use" comes from: the kernel
	// refuses a second binding of the same host:port.
	ip4 := net.ParseIP(config.Host).To4()
	if err := unix.Bind(serverFD, &unix.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		log.Fatal("Error while binding IP and Port ", err)
		return err
	}

	// Listen flips the socket from active to passive. An active socket is one
	// you'd Connect out on; a passive one only accepts inbound connections and
	// never carries application data itself.
	//
	// The kernel now completes TCP handshakes on our behalf and parks each
	// finished connection in an accept queue, up to maxClients deep. Accept
	// below pulls from that queue.
	if err = unix.Listen(serverFD, maxClients); err != nil {
		log.Fatal("Error while trying to listen to server ", err)
		return err
	}

	//------------------------------Async IO-----------------------------------

	// A kqueue (macOS equivalent of epoll) is a kernel-side readiness queue.
	// We hand it a set of descriptors to watch, and it blocks until at least one
	// is ready, then hands back only those. That is what lets one thread serve
	// thousands of connections: we never poll idle sockets, and we never park
	// on a client that has nothing to say.
	//
	// Note it is itself a file descriptor, from the same table as the sockets.
	kqueueFD, err := unix.Kqueue()
	if err != nil {
		log.Fatal(err)
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Println("Error while closing kqueue ", err)
		}
	}(kqueueFD)

	// Watch the listening socket for readability. On a passive socket
	// "readable" does not mean bytes are waiting, it means the accept queue is
	// non-empty: at least one client has completed its handshake and Accept
	// will return immediately.
	//
	//   Ident  - which descriptor to watch
	//   Filter - what kind of readiness (EVFILT_READ: readable)
	//   Flags  - EV_ADD registers it, EV_ENABLE arms it for delivery
	socketServerEvent := unix.Kevent_t{
		Ident:  uint64(serverFD),
		Filter: unix.EVFILT_READ,
		Flags:  unix.EV_ADD | unix.EV_ENABLE,
	}

	// Register the server socket to kqueue
	if _, err = unix.Kevent(kqueueFD, []unix.Kevent_t{socketServerEvent}, nil, nil); err != nil {
		log.Fatal("Error while registering server socket ", err)
		return err
	}

	for eStatus.Load()&EngineStatus__SHUTTING_DOWN == 0 {
		if time.Now().After(expiryCronLastExecution.Add(config.ExpiryCronFrequency)) {
			core.DeleteExpiredKeys()
			expiryCronLastExecution = time.Now()
		}

		// Block until something is ready, then fill events[] with only the
		// descriptors that are. nevents is how many of them are valid; the rest
		// of the buffer is stale from previous iterations and must be ignored.
		nevents, err := unix.Kevent(kqueueFD, nil, events[:], nil)
		if err != nil {
			continue
		}

		// Claim the engine for this batch. The only legal transition into
		// BUSY is from a clean WAITING, so once the signal handler has
		// OR-ed in SHUTTING_DOWN this CAS can never succeed again and no
		// further work is picked up.
		if !eStatus.CompareAndSwap(EngineStatus__WAITING, EngineStatus__BUSY) {
			return nil
		}

		// Every ready descriptor is either the listening socket (a new client is
		// knocking) or one of the per-connection sockets (an existing client
		// sent us a command). Comparing against serverFD is what tells them
		// apart, and it is the only distinction the loop needs.
		for i := 0; i < nevents; i++ {
			eventFD := int(events[i].Ident)

			if eventFD == serverFD {
				// Accept pulls one completed connection off the queue and
				// returns a BRAND-NEW descriptor for it. This is the key idea:
				//
				//   serverFD - one per process, bound to host:port, carries no
				//              application data, lives for the whole run, and
				//              exists only to manufacture client sockets
				//   clientFD - one per connected client, identified by the full
				//              (src IP, src port, dst IP, dst port) tuple, and
				//              the thing we actually read commands from and
				//              write replies to
				//
				// serverFD is untouched by this and stays listening, which is
				// how the next client can arrive while this one is being
				// served. Two clients from the same host get different
				// clientFDs because their source ports differ.
				clientFD, _, err := unix.Accept(serverFD)
				if err != nil {
					log.Println("err", err)
					continue
				}

				// Per-connection state (notably the MULTI queue) keyed by the
				// descriptor, so a later event on this clientFD can find the
				// same Client back.
				connectedClients[clientFD] = core.NewClient(clientFD)

				// Same reasoning as serverFD: a client that opens a connection
				// and then sends a partial command must not be able to block
				// the reads of every other client.
				if err = unix.SetNonblock(clientFD, true); err != nil {
					return err
				}

				// Hand the new connection to kqueue as well. From here on we
				// hear about this client only through the event loop, and here
				// "readable" has its ordinary meaning: bytes are waiting.
				socketClientEvent := unix.Kevent_t{
					Ident:  uint64(clientFD),
					Filter: unix.EVFILT_READ,
					Flags:  unix.EV_ADD | unix.EV_ENABLE,
				}
				if _, err = unix.Kevent(kqueueFD, []unix.Kevent_t{socketClientEvent}, nil, nil); err != nil {
					log.Fatal("Error while registering client socket ", err)
				}
			} else {
				// An existing connection has data. Recover the Client we
				// stored at accept time so any in-progress transaction on it
				// is still there.
				comm := connectedClients[eventFD]
				if comm == nil {
					continue
				}
				cmds, err := readCommands(comm)
				if err != nil {
					// Read failure here is overwhelmingly EOF: the client hung
					// up. Close the descriptor (which also drops it from
					// kqueue) and discard its state. Closing is what lets the
					// kernel reuse this integer for a future clientFD.
					unix.Close(eventFD)
					delete(connectedClients, eventFD)
					continue
				}
				respond(cmds, comm)

			}
		}
		// Release the engine. If this fails, shutdown was requested while we
		// were mid-batch: clear BUSY so the handler knows we have drained,
		// and stop picking up work.
		if !eStatus.CompareAndSwap(EngineStatus__BUSY, EngineStatus__WAITING) {
			eStatus.Store(EngineStatus__SHUTTING_DOWN)
			return nil
		}
	}
	return nil
}

func WaitForSignal(wg *sync.WaitGroup, sigs chan os.Signal) {
	defer wg.Done()
	sig := <-sigs
	log.Println("Received signal ", sig)

	// Announce the shutdown immediately rather than waiting for an idle
	// window. OR-ing the bit in preserves BUSY, so the engine cannot claim
	// a new batch from here on but an in-flight one stays visible to us.
	eStatus.Or(EngineStatus__SHUTTING_DOWN)

	// Wait for any in-flight batch to drain. This only costs latency now:
	// the BUSY bit is one-way once SHUTTING_DOWN is set, so no amount of
	// engine activity can starve us.
	for eStatus.Load()&EngineStatus__BUSY != 0 {
		time.Sleep(time.Millisecond)
	}

	core.Shutdown()
	os.Exit(0)
}
