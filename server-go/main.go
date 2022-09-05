package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	addr = flag.String("addr", ":3456", "http service `address`")
	// Server URL https://github.com/sijms/go-ora#servers-url-options
	connection = flag.String("conn", "", "oracle database `connection string`")
	help       = flag.Bool("help", false, "show help")

	ProgramName string

	//go:embed html/assets/*.js html/assets/*.css
	//go:embed html/index.html html/favicon.ico
	htmlFS embed.FS
)

func serveHtml() {
	//fs := http.FileServer(http.Dir("../client/dist/"))
	contentStatic, _ := fs.Sub(htmlFS, "html")
	http.Handle("/", http.StripPrefix("/", http.FileServer(http.FS(contentStatic))))
}

func usage() {
	fmt.Println("Oracle Profiler, 只有一个二进制执行文件, 通过监视 .trc 文件实时抓取 SQL 语句,")
	fmt.Println("本程序需要连接数据库读取 session 信息并开关 tracing 功能, 无其它危险行为.")
	fmt.Println("(兼容 Oracle 10g+)")
	fmt.Println("")
	fmt.Println("Usage: oraprofiler -conn=CONNECTION_STRING [-addr=[IP]:PORT]")
	flag.PrintDefaults()
}

func main() {
	flag.Parse()

	if *connection == "" || *help {
		usage()
		return
	}

	if p, err := os.Executable(); err == nil {
		ProgramName = filepath.Base(p)
	} else {
		ProgramName = "oraprofiler"
	}

	server, err := newServer(*connection)
	if err != nil {
		log.Fatal(err)
	}
	go server.run()

	serveHtml()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(server, w, r)
	})

	log.Println("Listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
