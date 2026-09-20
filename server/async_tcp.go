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

var conClients = 0
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

func init() {
	eStatus.Store(EngineStatus__WAITING)
}

func RunAsyncTCPServer(wg *sync.WaitGroup) error {
	defer wg.Done()
	defer func() {
		eStatus.Store(EngineStatus__SHUTTING_DOWN)
	}()

	log.Println("Starting async tcp server on ", config.Host, config.Port)

	maxClients := 20000

	// Create KQUEUE Event Objects to hold events
	events := make([]unix.Kevent_t, maxClients)

	// Create a Socket
	serverFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal("Error while creating Socket ", err)
		return err
	}
	defer unix.Close(serverFD)

	if err = unix.SetNonblock(serverFD, true); err != nil {
		return err
	}

	// Bind the IP and the port
	ip4 := net.ParseIP(config.Host).To4()
	if err := unix.Bind(serverFD, &unix.SockaddrInet4{
		Port: config.Port,
		Addr: [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]},
	}); err != nil {
		log.Fatal("Error while binding IP and Port ", err)
		return err
	}

	// Start listening
	if err = unix.Listen(serverFD, maxClients); err != nil {
		log.Fatal("Error while trying to listen to server ", err)
		return err
	}

	//------------------------------Async IO-----------------------------------

	// Creating Kqueue instance (macOS equivalent of epoll_create1) [cite: 10, 11]
	kqueueFD, err := unix.Kqueue()
	if err != nil {
		log.Fatal(err)
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Fatal("Error while closing kqueue ", err)
		}
	}(kqueueFD)

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

		for i := 0; i < nevents; i++ {
			eventFD := int(events[i].Ident)

			if eventFD == serverFD {
				clientFD, _, err := unix.Accept(serverFD)
				if err != nil {
					log.Println("err", err)
					continue
				}

				conClients += 1
				if err = unix.SetNonblock(clientFD, true); err != nil {
					return err
				}

				// Add this new TCP connection to be monitored
				socketClientEvent := unix.Kevent_t{
					Ident:  uint64(clientFD),
					Filter: unix.EVFILT_READ,
					Flags:  unix.EV_ADD | unix.EV_ENABLE,
				}
				if _, err = unix.Kevent(kqueueFD, []unix.Kevent_t{socketClientEvent}, nil, nil); err != nil {
					log.Fatal("Error while registering client socket ", err)
				}
			} else {
				// Kqueue gives us a handy EOF flag we can check right away for disconnects
				// TODO: implement graceful shutdown as this method hinders pipelined commands from running
				//if events[i].Flags&unix.EV_EOF != 0 {
				//	log.Println("Client socket closed")
				//	unix.Close(eventFD)
				//	conClients -= 1
				//	continue
				//}

				comm := core.FDComm{Fd: eventFD}
				cmds, err := readCommands(comm)
				if err != nil {
					unix.Close(eventFD)
					conClients -= 1
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
