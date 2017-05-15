package main

import (
	"flag"
	"fmt"
	"github.com/solar3s/goregen/regenbox"
	"github.com/solar3s/goregen/www"
	"jdid.co/bitbot/util"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
)

var (
	conn   *regenbox.SerialConnection
	server *www.Server
	rbCfg  regenbox.Config
	static string
)

var (
	device  = flag.String("dev", "", "path to serial port, if empty it will be searched automatically")
	root    = flag.String("root", "", "path to goregen's config files, defaults to $HOME/goregen")
	cfg     = flag.String("config", "", "path to config, defaults to <root>/config.toml")
	verbose = flag.Bool("verbose", false, "higher verbosity")
	debug   = flag.Bool("debug", false, "enable debug mode")

	winx = flag.Duration("winx", time.Millisecond*50, "patch duration for buggy windox read-flusher")
)

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func init() {
	flag.Parse()
	if *device != "" {
		port, config, err := regenbox.OpenPortName(*device)
		if err != nil {
			log.Fatal("error opening serial port: ", err)
		}
		conn = regenbox.NewSerial(port, config, *device)
		conn.Start()
	}

	if *root == "" {
		*root = filepath.Join(UserHomeDir(), "goregen")
	}
	for _, v := range []string{*root} {
		err := os.MkdirAll(v, 0755)
		if err != nil {
			log.Fatalf("couldn't mkdir \"%s\": %s", v, err)
		}
	}

	if *cfg == "" {
		*cfg = filepath.Join(*root, "config.toml")
	}

	rbCfg = regenbox.DefaultConfig
	err := util.ReadTomlFile(&rbCfg, *cfg)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("error reading config \"%s\": %s", *cfg, err)
		}
		err = util.WriteTomlFile(rbCfg, *cfg)
		if err != nil {
			log.Fatalf("error creating config \"%s\": %s", *cfg, err)
		}
		log.Printf("created new config file \"%s\"", *cfg)
	}

	// restore static assets
	static = filepath.Join(*root, "static")
	err = www.RestoreAssets(*root, "static")
	if err != nil {
		log.Fatalf("couldn't restore static assets in \"%s\": %s", static, err)
	}

	log.Printf("root directory: %s", *root)
	log.Printf("static directory: %s", static)
	log.Printf("config file: %s", *cfg)
}

func main() {
	rbox, err := regenbox.NewRegenBox(conn, &rbCfg)
	if err == regenbox.ErrNoSerialPortFound {
		log.Println("no regenbox detected")
	} else if err != nil {
		log.Println("error initializing regenbox connection:", err)
	}
	if rbox.Conn == nil {
		rbox.Conn = &regenbox.SerialConnection{
			WinxPatch:    *winx,
			ReadTimeout:  regenbox.DefaultTimeout,
			WriteTimeout: regenbox.DefaultTimeout,
		}
	}

	server = &www.Server{
		ListenAddr: "localhost:3636",
		Regenbox:   rbox,
		Verbose:    *verbose,
		Debug:      *debug,
		RboxConfig: *cfg,
		RootDir:    *root,
		StaticDir:  static,
		WsInterval: time.Second * 5,
	}
	server.Start()

	trap := make(chan os.Signal)
	signal.Notify(trap, os.Kill, os.Interrupt)
	sig := <-trap
	fmt.Println()
	log.Printf("signal: %s", sig.String())
	log.Println("stopping regenbox")
	server.Regenbox.Stop()
}
