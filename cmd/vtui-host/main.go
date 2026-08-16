package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/unxed/vtinput"
	"github.com/unxed/vtui"
)

func main() {
	protoFDFlag := flag.String("protocol-fd", "", "File descriptor number for protocol communication")
	socketFlag := flag.String("socket", "", "Unix domain socket path for protocol communication")
	backendFlag := flag.String("backend", "ansi", "Rendering backend (ansi, gogpu, x11, wayland, ebiten)")
	flag.Parse()

	if *protoFDFlag != "" {
		fd, err := strconv.Atoi(*protoFDFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vtui-host: invalid --protocol-fd: %v\n", err)
			os.Exit(1)
		}
		f := os.NewFile(uintptr(fd), "protocol_stream")
		defer f.Close()
		runSession(f, f, *backendFlag)
		return
	}

	if *socketFlag != "" {
		conn, err := net.Dial("unix", *socketFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vtui-host: failed to connect to socket %s: %v\n", *socketFlag, err)
			os.Exit(1)
		}
		defer conn.Close()
		runSession(conn, conn, *backendFlag)
		return
	}

	// Default to stdin/stdout
	runSession(os.Stdin, os.Stdout, *backendFlag)
}

func runSession(in io.Reader, out io.Writer, backend string) {
	// Guard against parent process exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigChan
		vtui.FrameManager.Shutdown()
		os.Exit(0)
	}()

	w, h, err := vtui.GetTerminalSize()
	if err != nil || w <= 0 || h <= 0 {
		w, h = 80, 25
	}

	scr := vtui.NewScreenBuf()
	scr.AllocBuf(w, h)
	vtui.FrameManager.Init(scr)
	vtui.FrameManager.SetHostMode(true)

	session := vtui.NewProtocolSession(in, out, vtui.FrameManager)

	go func() {
		_ = session.Serve()
		vtui.FrameManager.Shutdown()
	}()

	if backend != "ansi" && backend != "" {
		_ = vtui.RunInGUIWindow(w, h, backend, "", 18.0, func() {})
	} else {
		restore, err := vtinput.Enable()
		if err == nil && restore != nil {
			defer restore()
		}
		reader := vtinput.NewReader(os.Stdin, false)
		vtui.FrameManager.Run(reader)
	}
}
