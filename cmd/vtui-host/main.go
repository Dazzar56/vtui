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

	scr := vtui.NewScreenBuf()
	scr.AllocBuf(80, 25)
	vtui.FrameManager.Init(scr)

	session := vtui.NewProtocolSession(in, out, vtui.FrameManager)

	if backend != "ansi" && backend != "" {
		go func() {
			_ = session.Serve()
		}()
		_ = vtui.RunInGUIWindow(80, 25, backend, "", 18.0, func() {})
	} else {
		_ = session.Serve()
	}
}
