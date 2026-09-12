//go:build !fast
// +build !fast

package imagemounter_test

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/elazarl/goproxy"
	"github.com/stretchr/testify/assert"
)

func TestUsesProxy(t *testing.T) {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	// One signal per CONNECT, buffered so extra CONNECTs cannot panic the
	// way wg.Done() without a matching Add did.
	connected := make(chan struct{}, 8)

	proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		log.Printf("Got request for %s", host)
		select {
		case connected <- struct{}{}:
		default:
		}
		return goproxy.OkConnect, host
	})

	go func() {
		log.Print(http.ListenAndServe(":60001", proxy))
	}()
	tempDir, err := os.MkdirTemp("", "example")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %v\n", err)
		t.Fail()
		return
	}
	defer os.RemoveAll(tempDir)
	t.Cleanup(func() { _ = ios.UseHttpProxy("") })
	ios.UseHttpProxy("http://localhost:60001")
	path, err := imagemounter.Download17Plus(tempDir, ios.IOS17())
	if !assert.Nil(t, err) {
		t.Fail()
	}
	log.Printf("Downloaded to %s", path)
	select {
	case <-connected:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for first proxy CONNECT")
	}
	d, _ := ios.ListDevices()
	if len(d.DeviceList) == 0 {
		t.Skip("No device attached")
		return
	}
	m, err := imagemounter.NewPersonalizedDeveloperDiskImageMounter(d.DeviceList[0], ios.IOS17())
	if !assert.Nil(t, err) {
		t.Fail()
	}

	err = m.MountImage(path)
	if !assert.Nil(t, err) {
		t.Fail()
	}
	select {
	case <-connected:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for mount-path proxy CONNECT")
	}
}

func TestWorksWithoutProxy(t *testing.T) {

	tempDir, err := os.MkdirTemp("", "example")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)
	t.Cleanup(func() { _ = ios.UseHttpProxy("") })
	ios.UseHttpProxy("")
	path, err := imagemounter.Download17Plus(tempDir, ios.IOS17())
	if !assert.Nil(t, err) {
		t.Fail()
	}
	log.Printf("Downloaded to %s", path)

	d, _ := ios.ListDevices()
	if len(d.DeviceList) == 0 {
		t.Skip("No device attached")
		return
	}
	m, err := imagemounter.NewPersonalizedDeveloperDiskImageMounter(d.DeviceList[0], ios.IOS17())
	if !assert.Nil(t, err) {
		t.Fail()
	}

	err = m.MountImage(path)
	if !assert.Nil(t, err) {
		t.Fail()
	}

}
