package pkg_test

import (
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/enorith/fpm-proxyer/pkg"
)

func TestFpmSelect(t *testing.T) {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	fpm := pkg.NewFPM(19001, 5, BinPath)

	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	fpm.Select()
	<-done

	fpm.Close()
}
