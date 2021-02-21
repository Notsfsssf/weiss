package weiss

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"github.com/elazarl/goproxy"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	OneZeroCache = struct {
		Data map[string]string
		Lock sync.RWMutex
	}{make(map[string]string), sync.RWMutex{}}
	server *http.Server
)

func Start(port string) {
	configCache := make(map[string]*tls.Config, 0)
	DELAY := 8 * time.Second
	whiteList := []string{
		"pixiv.net",
		"www.pixiv.net",
		"app-api.pixiv.net",
		"oauth.secure.pixiv.net",
		"source.pixiv.net",
		"accounts.pixiv.net",
		"touch.pixiv.net",
		"imgaz.pixiv.net",
		"dic.pixiv.net",
		"comic.pixiv.net",
		"factory.pixiv.net",
		"g-client-proxy.pixiv.net",
		"sketch.pixiv.net",
		"payment.pixiv.net",
		"sensei.pixiv.net",
		"novel.pixiv.net",
		"en-dic.pixiv.net",
		"i1.pixiv.net",
		"i2.pixiv.net",
		"i3.pixiv.net",
		"i4.pixiv.net",
		"d.pixiv.org",
		"fanbox.pixiv.net",
		"pixivsketch.net",
		"pximg.net",
		"i.pximg.net",
		"s.pximg.net",
		"pixiv.pximg.net",
	}
	blackList := []string{}
	whitePorts := make([]string, len(whiteList))
	blackPorts := make([]string, len(blackList))
	for i, s := range whiteList {
		whitePorts[i] = s + ":443"
	}
	for i, s := range blackList {
		blackPorts[i] = s + ":443"
	}

	//go func() {
	//	for i := range whitePorts {
	//		domain := whiteList[i]
	//		req := OneZeroReq{
	//			name: domain,
	//		}
	//		data, err := req.PrePare()
	//		if err != nil {
	//			continue
	//		}
	//		OneZeroCache.Lock.Lock()
	//		OneZeroCache.Data[domain] = *data
	//		log.Println(*data)
	//		OneZeroCache.Lock.Unlock()
	//	}
	//}()
	//goproxy.ReqHostMatches()

	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest(
		goproxy.ReqHostIs(whitePorts...),
	).HijackConnect(func(req *http.Request, conn net.Conn, ctx *goproxy.ProxyCtx) {
		defer func() {
			if recover := recover(); recover != nil {
				_, _ = conn.Write([]byte("HTTP/1.1 500"))
			}
			conn.Close()
		}()
		log.Println(ctx.Req.URL.Hostname())
		clientTLSConfig, err := func(host string) (*tls.Config, error) {
			if config, ok := configCache[host]; ok {
				return config, nil
			}
			config, err := goproxy.TLSConfigFromCA(&goproxy.GoproxyCa)(host, ctx)
			if err != nil {
				return nil, err
			}
			configCache[host] = config
			return config, nil
		}(ctx.Req.URL.Host)
		if err != nil {
			panic(err)
		}
		tlsCon := tls.Server(conn, clientTLSConfig)
		_ = tlsCon.SetDeadline(time.Now().Add(DELAY))
		if err := tlsCon.Handshake(); err != nil {
			panic(err)
		}
		defer tlsCon.Close()
		clientWriter := bufio.NewReadWriter(bufio.NewReader(tlsCon), bufio.NewWriter(tlsCon))
		remoteCon := buildOneZeroCon(ctx)
		if remoteCon == nil {
			panic("Error host:" + ctx.Req.URL.Hostname())
		}
		defer remoteCon.Close()
		remote := tls.Client(remoteCon, &tls.Config{
			InsecureSkipVerify: true,
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				return nil //先不听鱼的
			},
		})
		if err := remote.Handshake(); err != nil {
			panic(err)
		}
		defer remote.Close()
		remoteWriter := bufio.NewReadWriter(bufio.NewReader(remote), bufio.NewWriter(remote))
		channel := make(chan error)
		go func() {
			buffer := make([]byte, 1024)
			var err error
			for {
				_ = tlsCon.SetDeadline(time.Now().Add(DELAY))
				num, err := clientWriter.Read(buffer)
				if err != nil {
					break
				}
				_, err = remoteWriter.Write(buffer[:num])
				if err != nil {
					break
				}
				if err := remoteWriter.Flush(); err != nil {
					break
				}
			}
			channel <- err
		}()
		go func() {
			buffer := make([]byte, 1024)
			var err error
			for {
				_ = tlsCon.SetDeadline(time.Now().Add(DELAY))
				num, err := remoteWriter.Read(buffer)
				if err != nil {
					break
				}
				_, err = clientWriter.Write(buffer[:num])
				if err != nil {
					break
				}
				if err := clientWriter.Flush(); err != nil {
					break
				}
			}
			channel <- err
		}()
		if err := <-channel; err != nil {
			panic(err)
		}
		if err := <-channel; err != nil {
			panic(err)
		}
	})
	proxy.OnRequest(
		goproxy.ReqHostIs(blackPorts...),
	).HandleConnect(goproxy.AlwaysReject)
	proxy.Verbose = true
	server = &http.Server{Addr: ":" + port, Handler: proxy}
	go func() {
		if err := server.ListenAndServe(); err != nil {
		}
	}()
}

func Close() {
	if server != nil {
		_ = server.Close()
	}
}

func buildOneZeroCon(ctx *goproxy.ProxyCtx) net.Conn {
	OneZeroCache.Lock.RLock()
	data, ok := OneZeroCache.Data[ctx.Req.URL.Hostname()]
	OneZeroCache.Lock.RUnlock()
	if ok {
		remoteCon, err := net.Dial("tcp", data+ctx.Req.Host[strings.LastIndex(ctx.Req.Host, ":"):])
		if err != nil {
			return nil
		}
		return remoteCon
	}
	oneZeroReq := OneZeroReq{
		ctx.Req.URL.Hostname(),
	}
	res, err := oneZeroReq.fetch()
	if err != nil {
		panic(err)
	}
	for _, answer := range res.Answer {
		if answer.Type != 1 {
			continue
		}
		remoteCon, err := net.Dial("tcp", answer.Data+ctx.Req.Host[strings.LastIndex(ctx.Req.Host, ":"):])
		if err != nil {
		}
		OneZeroCache.Lock.Lock()
		OneZeroCache.Data[ctx.Req.URL.Hostname()] = answer.Data
		OneZeroCache.Lock.Unlock()
		return remoteCon
	}
	return nil
}
