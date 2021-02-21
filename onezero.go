package weiss

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
)

type OneZeroReq struct {
	name string
}

type OneZeroRes struct {
	Status   int        `json:"Status"`
	TC       bool       `json:"TC"`
	RD       bool       `json:"RD"`
	RA       bool       `json:"RA"`
	AD       bool       `json:"AD"`
	CD       bool       `json:"CD"`
	Question []Question `json:"Question"`
	Answer   []Answer   `json:"Answer"`
}

type Question struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type Answer struct {
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
}

var (
	hardcodeIpMap = make(map[string]string)
	PIXIV_API_IP  = "210.140.131.199"
)

func init() {
	hardcodeIpMap["app-api.pixiv.net"] = "210.140.131.199"
	hardcodeIpMap["oauth.secure.pixiv.net"] = "210.140.131.199"
}

func (oneZeroReq *OneZeroReq) fetch() (*OneZeroRes, error) {
	url := fmt.Sprintf("https://cloudflare-dns.com/dns-query?ct=application/dns-json&name=%s&type=A&do=false&cd=false", oneZeroReq.name)
	log.Print(url)
	v, ok := hardcodeIpMap[oneZeroReq.name]
	answer := &OneZeroRes{}
	if ok {
		answer.Answer = []Answer{{Data: v, TTL: 50, Type: 1}}
		return answer, nil
	}
	var body []byte
	var err error
	for i := 0; i < 3; i++ {
		body, err = request(url)
		if err == nil {
			break
		} else if i == 2 {
			body, err = request(strings.ReplaceAll(url, "cloudflare-dns.com", "1.0.0.1"))
		}
	}
	err = json.Unmarshal(body, answer)
	if err != nil {
		answer.Answer = []Answer{{Data: PIXIV_API_IP, TTL: 50, Type: 1}}
		return answer, err
	}
	for i := range answer.Answer {
		print(answer.Answer[i].Data)
		if strings.Contains(answer.Answer[i].Data, "104") {
			answer.Answer[i].Data = PIXIV_API_IP
		}
	}
	return answer, err
}

func request(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func (oneZeroReq *OneZeroReq) PrePare() (*string, error) {
	res, err := oneZeroReq.fetch()
	if err != nil {
		return nil, err
	}
	for _, item := range res.Answer {
		if item.Type != 1 {
			continue
		}
		conn, err := net.Dial("tcp", item.Data+":443")
		if err != nil {
			continue
		}
		OneZeroCache.Data[oneZeroReq.name] = item.Data
		conn.Close()
		return &item.Data, nil
		break
	}
	return nil, err
}
