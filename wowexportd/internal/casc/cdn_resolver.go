package casc

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type CDNResolver struct{}

func (CDNResolver) Ping(host string) (time.Duration, error) {
	t0 := time.Now()
	c := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("HEAD", host, nil)
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return time.Since(t0), nil
}

func (CDNResolver) GetBestHost(region string, hosts string, path string) (string, error) {
	arr := strings.Split(hosts, " ")
	if len(arr) == 0 {
		return "", fmt.Errorf("no hosts")
	}
	type hp struct {
		host string
		ping time.Duration
	}
	var list []hp
	for _, h := range arr {
		url := "https://" + h + "/"
		if d, err := (CDNResolver{}).Ping(url); err == nil {
			list = append(list, hp{url, d})
		}
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no reachable hosts")
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ping < list[j].ping })
	return list[0].host + path + "/", nil
}
