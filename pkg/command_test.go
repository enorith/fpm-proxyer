package pkg_test

import (
	"log"
	"testing"
	"time"

	"github.com/enorith/fpm-proxyer/pkg"
)

const BinPath = "C:\\Users\\Nerio\\php"

func TestCommandExec(t *testing.T) {
	c := pkg.NewCommand("php-cgi.exe -b 127.0.0.1:19001", log.Default())

	c.GoExec(BinPath)

	time.Sleep(5 * time.Second)

	t.Error(c.Stop())
}
