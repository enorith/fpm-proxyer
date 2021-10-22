package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CaoJiayuan/messenger"
	"github.com/enorith/config"
	"github.com/enorith/container"
	"github.com/enorith/fpm-proxyer/pkg"
	"github.com/enorith/fpm-proxyer/views"
	enorith "github.com/enorith/http"
	"github.com/enorith/http/content"
	"github.com/enorith/http/contracts"
	"github.com/enorith/http/router"
	"github.com/enorith/http/view"
	"github.com/urfave/cli/v2"
	"github.com/yookoala/gofast"
)

const BinPath = "C:\\Users\\Nerio\\php"

type ServerInfo struct {
	Address string `json:"address"`
	Serving int    `json:"serving"`
	Pid     int    `json:"pid"`
	Running bool   `json:"running"`
}

type Report struct {
	Info []ServerInfo `json:"info"`
}

type Config struct {
	Listen      string `yaml:"listen"`
	Dashboard   string `yaml:"dashboard"`
	BinPath     string `yaml:"binPath"`
	StartPort   int    `yaml:"startPort"`
	MaxProcess  int    `yaml:"maxProcess"`
	IdelProcess int    `yaml:"idelProcess" default:"3"`
	Timeout     int    `yaml:"timeout" default:"200"`
}

func main() {
	app := cli.NewApp()
	var cfg string
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Usage:       "Load configuration from `FILE`",
			Destination: &cfg,
		},
	}
	app.Description = "FPM for windows"
	app.Action = func(c *cli.Context) error {
		if cfg == "" {
			return cli.Exit(fmt.Sprintf("Usage: %s -c config.yml", c.App.Name), 1)
		}
		var cf Config

		config.Unmarshal(cfg, &cf)

		run(cf)

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(cfg Config) {
	listen := cfg.Listen
	l, e := net.Listen("tcp", listen)
	if e != nil {
		log.Fatal(e)
	}
	dashboard := cfg.Dashboard

	logger := log.Default()
	logger.Printf("listen [%s], dashboard [%s], binPath [%s], startPort [%d], maxProcess [%d], idelProcess [%d]", listen, dashboard, cfg.BinPath,
		cfg.StartPort, cfg.MaxProcess, cfg.IdelProcess)

	fpm := pkg.NewFPM(cfg.StartPort, cfg.MaxProcess, cfg.BinPath)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		err := fcgi.Serve(l, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			// logger.Printf("%s %s", r.Method, r.RequestURI)

			env := fcgi.ProcessEnv(r)
			p := fpm.Select()
			//logger.Printf("CGI address [%s], serving [%d], root %s", p.Address(), p.Serving(), env["DOCUMENT_ROOT"])
			r.RequestURI = r.URL.Path

			h := newFastHandler("tcp", p.Address(), env)
			t := time.NewTimer(time.Duration(cfg.Timeout) * time.Second)
			exit := make(chan struct{}, 1)
			go func() {
				h.ServeHTTP(rw, r)
				exit <- struct{}{}
			}()
		loop:
			for {
				select {
				case <-t.C:
					logger.Printf("cgi handle timeout [%ds], addr %s, pid %d", cfg.Timeout, p.Address(), p.Pid())
					rw.WriteHeader(504)
					_, e := rw.Write([]byte("cgi gateway timeout"))
					if e != nil {
						logger.Println(e)
					}
					p.Stop()
					break loop
				case <-exit:
					p.Served()
					break loop
				}
			}
		}))
		if err != nil {
			logger.Fatal(err)
		}
	}()
	defer fpm.Close()

	srv, _ := messenger.NewServer()
	srv.Cors()
	srv.RegisterEvents().ServeIo()
	defer srv.Close()

	go func() {
		for {
			<-time.After(1 * time.Second)
			reportServer(srv, fpm)
		}
	}()
	fpm.PruneInterval(5*time.Second, cfg.IdelProcess)

	server := enorith.NewServer(func(request contracts.RequestContract) container.Interface {
		return container.New()
	}, true)

	view.WithDefault(views.FS, "html")

	server.Serve(dashboard, func(rw *router.Wrapper, k *enorith.Kernel) {
		k.Handler = enorith.HandlerNetHttp
		rw.Get("/", func() (*content.TemplateResponse, error) {

			return view.View("dashboard", 200, nil)
		})
		rw.RegisterAction(router.ANY, "/socket.io", srv)
	})
}

func reportServer(srv *messenger.Server, fpm *pkg.FPM) {
	info := make([]ServerInfo, 0)

	for _, p := range fpm.Cgis() {
		info = append(info, ServerInfo{
			Address: p.Address(),
			Serving: p.Serving(),
			Pid:     p.Pid(),
			Running: p.Running(),
		})
	}
	srv.Broadcast("message", Report{
		Info: info,
	}, "report")
}

func newFastHandler(network, address string, env map[string]string) gofast.Handler {
	connFactory := gofast.SimpleConnFactory(network, address)
	docRoot := env["DOCUMENT_ROOT"]
	handler := gofast.NewPHPFS(docRoot)(gofast.BasicSession)

	return gofast.NewHandler(FPMParamMap(docRoot, env)(handler),
		gofast.SimpleClientFactory(connFactory))
}

//FPMParamMap Fast-CGI 参数构造
func FPMParamMap(docRoot string, env map[string]string) gofast.Middleware {
	return func(inner gofast.SessionHandler) gofast.SessionHandler {
		return func(client gofast.Client, req *gofast.Request) (resp *gofast.ResponsePipe, err error) {
			resp, err = inner(client, req)

			req.Params["DOCUMENT_ROOT"] = docRoot
			req.Params["SCRIPT_FILENAME"] = filepath.Join(docRoot, "index.php")
			req.Params["SCRIPT_NAME"] = "index.php"
			// req.Params["REQUEST_URI"] = env["DOCUMENT_URI"]

			return
		}
	}
}
