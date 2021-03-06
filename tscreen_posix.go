// +build solaris

// Copyright 2017 The TCell Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcell

import (
	"fmt"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
	"os"
	"os/signal"
	"syscall"
)

type termiosPrivate syscall.Termios

var tiosp = termiosPrivate{}

func (t *tScreen) termioInit() (err error) {
	private := syscall.Termios{}

	if t.in, err = os.OpenFile("/dev/tty", os.O_RDONLY, 0); err != nil {
		return err
	}
	if t.out, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if t.in != nil {
				t.in.Close()
			}
			if t.out != nil {
				t.out.Close()
			}
		}
	}()

	fd := t.out.Fd()

	if err := termios.Tcgetattr(fd, &private); err != nil {
		return fmt.Errorf("cannot get attributes: %s", err)
	}

	t.baud = int(termios.Cfgetospeed(&private))

	private.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK |
		syscall.ISTRIP | syscall.INLCR | syscall.IGNCR |
		syscall.ICRNL | syscall.IXON
	private.Oflag &^= syscall.OPOST
	private.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON |
		syscall.ISIG | syscall.IEXTEN
	private.Cflag &^= syscall.CSIZE | syscall.PARENB
	private.Cflag |= syscall.CS8

	// This is setup for blocking reads.  In the past we attempted to
	// use non-blocking reads, but now a separate input loop and timer
	// copes with the problems we had on some systems (BSD/Darwin)
	// where close hung forever.
	private.Cc[syscall.VMIN] = 1
	private.Cc[syscall.VTIME] = 0

	if err = termios.Tcsetattr(fd, termios.TCSAFLUSH|termios.TCSANOW, &private); err != nil {
		return fmt.Errorf("cannot set attributes: %s", err)
	}

	signal.Notify(t.sigwinch, syscall.SIGWINCH)

	if w, h, e := t.getWinSize(); e == nil && w != 0 && h != 0 {
		t.cells.Resize(w, h)
	}

	tiosp = termiosPrivate(private)
	t.tiosp = &tiosp

	return nil
}

func (t *tScreen) termioFini() {

	signal.Stop(t.sigwinch)

	<-t.indoneq

	private := syscall.Termios(tiosp)

	if t.out != nil {
		termios.Tcsetattr(t.out.Fd(), termios.TCSETS|termios.TCSETSF, &private)
		t.out.Close()
	}
	if t.in != nil {
		t.in.Close()
	}
}

func (t *tScreen) getWinSize() (int, int, error) {
	winsize, err := unix.IoctlGetWinsize(int(t.out.Fd()), syscall.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(winsize.Col), int(winsize.Row), nil
}
