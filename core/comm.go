package core

import "golang.org/x/sys/unix"

type FDComm struct {
	Fd int
}

func (f FDComm) Read(b []byte) (int, error) {
	return unix.Read(f.Fd, b)
}

func (f FDComm) Write(b []byte) (int, error) {
	return unix.Write(f.Fd, b)
}
