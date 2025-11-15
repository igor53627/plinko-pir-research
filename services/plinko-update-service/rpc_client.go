package main

import (
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type authTransport struct {
	token string
	base  http.RoundTripper
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	return a.base.RoundTrip(req)
}

func dialEthereumClient(url, token string) (*ethclient.Client, error) {
	if token == "" || !strings.HasPrefix(url, "http") {
		return ethclient.Dial(url)
	}

	httpClient := &http.Client{
		Transport: &authTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}

	rpcClient, err := rpc.DialHTTPWithClient(url, httpClient)
	if err != nil {
		return nil, err
	}

	return ethclient.NewClient(rpcClient), nil
}
