package server

import (
	"log"
	"net"
	"redis-go/config"
	"redis-go/core"
	"time"

	"golang.org/x/sys/unix"
)

var conClients = 0
var expiryCronLastExecution = time.Now()

func RunAsyncTCPServer() error {
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
	defer unix.Close(kqueueFD)

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

	for {
		if time.Now().After(expiryCronLastExecution.Add(config.ExpiryCronFrequency)) {
			core.DeleteExpiredKeys()
			expiryCronLastExecution = time.Now()
		}

		nevents, err := unix.Kevent(kqueueFD, nil, events[:], nil)
		if err != nil {
			continue
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
				if events[i].Flags&unix.EV_EOF != 0 {
					log.Println("Client socket closed")
					unix.Close(eventFD)
					conClients -= 1
					continue
				}

				comm := core.FDComm{Fd: eventFD}
				cmd, err := readCommand(comm)
				if err != nil {
					unix.Close(eventFD)
					conClients -= 1
					continue
				}
				respond(cmd, comm)

			}
		}
	}

}
